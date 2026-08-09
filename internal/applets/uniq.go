package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

func newUniqApplet() Applet {
	return uniqApplet{}
}

type uniqApplet struct{}

func (uniqApplet) Name() string {
	return "uniq"
}

func (uniqApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	input, err := parseUniqArgs(args)
	if err != nil {
		return writeUniqDiagnostic(stderr, err.Error())
	}
	lines, err := readUniqInput(ctx, ProcessViewFromContext(ctx), input, stdin)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return writeUniqDiagnostic(stderr, inputDiagnostic("uniq", err))
	}
	return writeUniqLines(stdout, collapseAdjacentLines(lines, input.count))
}

type uniqInput struct {
	path    string
	hasPath bool
	// count prefixes each run with how many lines it collapsed, which is what
	// makes `sort | uniq -c | sort -rn` -- the tally everyone writes -- work.
	count bool
}

func parseUniqArgs(args []string) (uniqInput, error) {
	var operands []string
	count := false
	for index := range len(args) {
		arg := args[index]
		if arg == "--" {
			operands = append(operands, args[index+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			operands = append(operands, arg)
			continue
		}
		// See sort: a long option read letter by letter names the `-` it begins
		// with rather than the option that was actually typed.
		if strings.HasPrefix(arg, "--") {
			return uniqInput{}, fmt.Errorf("uniq: unrecognized option %s", arg)
		}
		for _, flag := range arg[1:] {
			if flag == 'c' {
				count = true
				continue
			}
			return uniqInput{}, fmt.Errorf("uniq: invalid option -- %c", flag)
		}
	}

	if len(operands) > 1 {
		return uniqInput{}, errors.New("uniq: too many operands")
	}
	if len(operands) == 0 || operands[0] == "-" {
		return uniqInput{count: count}, nil
	}
	return uniqInput{path: operands[0], hasPath: true, count: count}, nil
}

func readUniqInput(ctx context.Context, view ProcessView, input uniqInput, stdin io.Reader) ([]string, error) {
	if !input.hasPath {
		return readUniqLines(stdin)
	}
	reader, err := OpenProcessInput(ctx, view, input.path)
	if err != nil {
		return nil, quotedInputFailure(input.path, err)
	}
	lines, readErr := readUniqLines(reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, inputFailure(input.path, err)
	}
	return lines, nil
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

// collapseAdjacentLines keeps one of each run of equal lines, and under -c
// prefixes it with the size of the run.
//
// The layout is busybox's, which is GNU's: the count right-aligned in seven
// columns and then a blank, so a column of tallies lines up.
func collapseAdjacentLines(lines []string, count bool) []string {
	if len(lines) == 0 {
		return nil
	}
	collapsed := make([]string, 0, len(lines))
	previous := ""
	run := 0
	flush := func() {
		if run == 0 {
			return
		}
		if count {
			collapsed = append(collapsed, fmt.Sprintf("%7d %s", run, previous))
			return
		}
		collapsed = append(collapsed, previous)
	}
	for index, line := range lines {
		if index > 0 && line == previous {
			run++
			continue
		}
		flush()
		previous, run = line, 1
	}
	flush()
	return collapsed
}

func writeUniqLines(stdout io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

// Like cut, uniq leaves xfunc_error_retval alone, so its xopen death and its
// bb_show_usage both exit 1 (coreutils/uniq.c:76 and :81).
func writeUniqDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(1)
}
