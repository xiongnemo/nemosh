package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func newUniqApplet() Applet {
	return uniqApplet{}
}

type uniqApplet struct{}

func (uniqApplet) Name() string {
	return "uniq"
}

// uniq is busybox's (coreutils/uniq.c): `uniq [-cduiz] [-f N] [-s N] [-w N] [INPUT [OUTPUT]]`.
// Adjacent lines are one run when what they compare is the same: past -f's first N fields and
// then -s's N characters, at most -w's N characters of what is left, ASCII case folded under -i.
// The first line of each run is written, after its size under -c, to OUTPUT when there is one;
// -z ends each with NUL, which busybox has for the output alone.
//
// It took -c -d -u -i and refused a second operand, where busybox writes the result to it.
func (uniqApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	input, err := parseUniqArgs(ctx, args)
	if err != nil {
		return writeUniqDiagnostic(stderr, err.Error())
	}
	view := ProcessViewFromContext(ctx)
	var opened io.ReadCloser
	reader := stdin
	if input.hasPath {
		file, err := OpenProcessInput(ctx, view, input.path)
		if err != nil {
			return writeUniqDiagnostic(stderr, inputDiagnostic("uniq", quotedInputFailure(input.path, err)))
		}
		opened, reader = file, file
	}
	// A third operand is refused once INPUT is open, as busybox's order has it.
	if input.extra != "" {
		if opened != nil {
			opened.Close()
		}
		return writeUniqDiagnostic(stderr, fmt.Sprintf("uniq: extra operand '%s'", input.extra))
	}
	// OUTPUT is opened before anything is read, as busybox opens it.
	var output io.WriteCloser
	if input.hasOutput {
		file, err := openUniqOutput(view, input.output)
		if err != nil {
			if opened != nil {
				opened.Close()
			}
			return writeUniqDiagnostic(stderr, "uniq: "+err.Error())
		}
		output = file
	}
	lines, err := readUniqLines(reader)
	if opened != nil {
		err = errors.Join(err, opened.Close())
	}
	if err != nil {
		if output != nil {
			output.Close()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return writeUniqDiagnostic(stderr, inputDiagnostic("uniq", inputFailure(input.path, err)))
	}
	if output == nil {
		return input.write(stdout, lines)
	}
	return errors.Join(input.write(output, lines), output.Close())
}

type uniqInput struct {
	// path and output are the operands; without one, or given `-`, it is standard input and
	// output. An empty operand is a file, which is not there.
	path, output       string
	hasPath, hasOutput bool
	// extra is a third operand, which is refused.
	extra string
	// count prefixes each run with how many lines it collapsed, which is what makes `sort |
	// uniq -c | sort -rn` -- the tally everyone writes -- work.
	count bool
	// onlyRepeated is -d and onlyUnique -u, opposites, so both at once select nothing.
	onlyRepeated, onlyUnique bool
	foldCase, zero           bool
	skipFields, skipChars    int
	// maxChars is -w, or -1 for the whole of what is compared.
	maxChars int
}

func parseUniqArgs(ctx context.Context, args []string) (uniqInput, error) {
	options, operands, err := parseAppletOptions(ctx, args, "cduiz", "fsw")
	if err != nil {
		return uniqInput{}, errors.New("uniq: " + err.Error())
	}
	parsed := uniqInput{
		count: options.has('c'), onlyRepeated: options.has('d'), onlyUnique: options.has('u'),
		foldCase: options.has('i'), zero: options.has('z'), maxChars: -1,
	}
	for _, number := range []struct {
		letter byte
		into   *int
	}{{'f', &parsed.skipFields}, {'s', &parsed.skipChars}, {'w', &parsed.maxChars}} {
		if !options.has(number.letter) {
			continue
		}
		value, err := strconv.ParseUint(options.value(number.letter), 10, 31)
		if err != nil {
			return uniqInput{}, fmt.Errorf("uniq: invalid number '%s'", options.value(number.letter))
		}
		*number.into = int(value)
	}
	if len(operands) > 2 {
		parsed.extra = operands[2]
	}
	if len(operands) > 0 && operands[0] != "-" {
		parsed.path, parsed.hasPath = operands[0], true
	}
	if len(operands) > 1 && operands[1] != "-" {
		parsed.output, parsed.hasOutput = operands[1], true
	}
	return parsed, nil
}

func openUniqOutput(view ProcessView, path string) (io.WriteCloser, error) {
	file, err := openProcessOutput(view, path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
	if err != nil {
		return nil, cannotOpen(path, err)
	}
	return file, nil
}

func readUniqLines(input io.Reader) ([]string, error) {
	reader := bufio.NewReader(input)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			lines = append(lines, line)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return lines, nil
		}
		return nil, err
	}
}

// write writes the first line of each run, as busybox's does: under -d only the runs that
// repeated, under -u only the ones that did not, and under -c after the run's size in seven
// columns and a blank, so a column of tallies lines up.
func (input uniqInput) write(out io.Writer, lines []string) error {
	end := byte('\n')
	if input.zero {
		end = 0
	}
	writer := bufio.NewWriter(out)
	for start := 0; start < len(lines); {
		next := start + 1
		for next < len(lines) && input.same(lines[start], lines[next]) {
			next++
		}
		repeated := next-start > 1
		if !(input.onlyRepeated && !repeated) && !(input.onlyUnique && repeated) {
			if input.count {
				fmt.Fprintf(writer, "%7d ", next-start)
			}
			writer.WriteString(lines[start])
			if err := writer.WriteByte(end); err != nil {
				return err
			}
		}
		start = next
	}
	return writer.Flush()
}

// same is whether two lines are one run: what they compare, past -f's fields and -s's
// characters, up to -w's.
func (input uniqInput) same(left, right string) bool {
	left, right = input.compared(left), input.compared(right)
	if input.foldCase {
		return asciiEqualFold(left, right)
	}
	return left == right
}

// compared is busybox's cur_compare: past each skipped field's blanks and then what follows them
// up to the next blank, past the skipped characters, and at most -w's of what is left.
func (input uniqInput) compared(line string) string {
	index := 0
	for range input.skipFields {
		for index < len(line) && isCSpace(line[index]) {
			index++
		}
		for index < len(line) && !isCSpace(line[index]) {
			index++
		}
	}
	line = line[min(index+input.skipChars, len(line)):]
	if input.maxChars >= 0 && input.maxChars < len(line) {
		line = line[:input.maxChars]
	}
	return line
}

// asciiEqualFold is strncasecmp's equality: ASCII letters folded, every other byte as it is.
func asciiEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range len(left) {
		a, b := left[index], right[index]
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// Like cut, uniq leaves xfunc_error_retval alone, so its xopen death and its
// bb_show_usage both exit 1 (coreutils/uniq.c:76 and :81).
func writeUniqDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(1)
}
