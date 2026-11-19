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
// -z ends each with NUL, and reads a NUL as the end of a line as a newline is; see
// readUniqLines.
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
	target := stdout
	if output != nil {
		target = output
	}
	readErr, writeErr := input.stream(reader, target)
	if opened != nil {
		readErr = errors.Join(readErr, opened.Close())
	}
	if output != nil {
		writeErr = errors.Join(writeErr, output.Close())
	}
	if readErr != nil {
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return readErr
		}
		return writeUniqDiagnostic(stderr, inputDiagnostic("uniq", inputFailure(input.path, readErr)))
	}
	return writeErr
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

// eachUniqLine hands on each line of input as it is read: split at newlines, a CR before one
// dropped, and under -z at NULs as well, as busybox's getline ends a line at either and its -z
// sets only what is written after one: `sort -z | uniq -z` is one line per name. Under -z a NUL
// ended nothing, so `printf 'a\0a\0b\0' | uniq -z` was a single line with a NUL added. busybox's
// getline ends a line at a NUL without -z too; that is not copied, as cut does not copy it.
//
// A line at a time, as busybox reads them: all of them were read first, so nothing was written
// until the input ended, and `uniq -d` of 19 MB peaked at 67 MiB where busybox's peaked at 1.1.
// waiting is called before a read that may wait, once what was read is used up.
func eachUniqLine(input io.Reader, zero bool, each func(string) error, waiting func() error) error {
	reader := bufio.NewReader(input)
	for {
		if reader.Buffered() == 0 {
			if err := waiting(); err != nil {
				return err
			}
		}
		chunk, err := reader.ReadString('\n')
		if chunk != "" {
			ended := strings.HasSuffix(chunk, "\n")
			chunk = strings.TrimSuffix(strings.TrimSuffix(chunk, "\n"), "\r")
			pieces := []string{chunk}
			if zero {
				pieces = strings.Split(chunk, "\x00")
				// A NUL last in the input ends the line before it, and begins none.
				if !ended && len(pieces) > 1 && pieces[len(pieces)-1] == "" {
					pieces = pieces[:len(pieces)-1]
				}
			}
			for _, piece := range pieces {
				if eachErr := each(piece); eachErr != nil {
					return eachErr
				}
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}

// stream writes the first line of each run, as busybox's does, once the next has begun: under -d
// only the runs that repeated, under -u only the ones that did not, and under -c after the run's
// size in seven columns and a blank, so a column of tallies lines up. A failure to read and one
// to write are answered apart, as only the first is uniq's to name.
func (input uniqInput) stream(in io.Reader, out io.Writer) (readErr, writeErr error) {
	end := byte('\n')
	if input.zero {
		end = 0
	}
	writer := bufio.NewWriter(out)
	first, size := "", 0
	emit := func() error {
		repeated := size > 1
		if input.onlyRepeated && !repeated || input.onlyUnique && repeated {
			return nil
		}
		if input.count {
			fmt.Fprintf(writer, "%7d ", size)
		}
		writer.WriteString(first)
		return writer.WriteByte(end)
	}
	readErr = eachUniqLine(in, input.zero, func(line string) error {
		if size > 0 && input.same(first, line) {
			size++
			return nil
		}
		if size > 0 {
			if writeErr = emit(); writeErr != nil {
				return writeErr
			}
		}
		first, size = line, 1
		return nil
	}, func() error {
		// What is written is out before a read that may wait, so `tail -f log | uniq` shows a
		// run as the next begins, and a file is written a buffer at a time, not a line.
		writeErr = writer.Flush()
		return writeErr
	})
	if writeErr != nil {
		return nil, writeErr
	}
	if readErr == nil && size > 0 {
		writeErr = emit()
	}
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	return readErr, writeErr
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
