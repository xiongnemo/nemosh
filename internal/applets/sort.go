package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	stdsort "sort"
	"strconv"
	"strings"
	"unicode"
)

func newSortApplet() Applet {
	return sortApplet{}
}

type sortApplet struct{}

func (sortApplet) Name() string {
	return "sort"
}

func (sortApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseSortArgs(args)
	if err != nil {
		return writeSortDiagnostic(stderr, err.Error())
	}
	lines, err := readSortInputs(ctx, ProcessViewFromContext(ctx), options, stdin)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return writeSortDiagnostic(stderr, sortInputError(err))
	}
	sortTextLines(lines, options)
	return writeSortLines(stdout, lines)
}

type sortOptions struct {
	numeric bool
	reverse bool
	paths   []string
}

func parseSortArgs(args []string) (sortOptions, error) {
	var options sortOptions
	for index := range len(args) {
		arg := args[index]
		if arg == "--" {
			options.paths = append(options.paths, args[index+1:]...)
			return options, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			options.paths = append(options.paths, arg)
			continue
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'n':
				options.numeric = true
			case 'r':
				options.reverse = true
			default:
				return sortOptions{}, fmt.Errorf("sort: invalid option -- %c", flag)
			}
		}
	}
	return options, nil
}

func readSortInputs(ctx context.Context, view ProcessView, options sortOptions, stdin io.Reader) ([]string, error) {
	if len(options.paths) == 0 {
		return readSortLines(stdin)
	}
	var lines []string
	for _, path := range options.paths {
		input, err := OpenProcessInput(ctx, view, path)
		if err != nil {
			return nil, sortReadError{path: path, err: err}
		}
		fileLines, readErr := readSortLines(input)
		closeErr := input.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, sortReadError{path: path, err: err}
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

type sortReadError struct {
	path string
	err  error
}

func (e sortReadError) Error() string {
	return e.err.Error()
}

func (e sortReadError) Unwrap() error {
	return e.err
}

func readSortLines(input io.Reader) ([]string, error) {
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

func sortTextLines(lines []string, options sortOptions) {
	stdsort.Slice(lines, func(i, j int) bool {
		comparison := compareSortLines(lines[i], lines[j], options)
		if options.reverse {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareSortLines(left string, right string, options sortOptions) int {
	if options.numeric {
		leftNumber := sortNumericPrefix(left)
		rightNumber := sortNumericPrefix(right)
		if leftNumber < rightNumber {
			return -1
		}
		if leftNumber > rightNumber {
			return 1
		}
	}
	return strings.Compare(left, right)
}

func sortNumericPrefix(line string) int64 {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if trimmed == "" {
		return 0
	}
	end := 0
	if trimmed[0] == '+' || trimmed[0] == '-' {
		end = 1
	}
	for end < len(trimmed) && trimmed[end] >= '0' && trimmed[end] <= '9' {
		end++
	}
	if end == 0 || (end == 1 && (trimmed[0] == '+' || trimmed[0] == '-')) {
		return 0
	}
	value, err := strconv.ParseInt(trimmed[:end], 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func writeSortLines(stdout io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

func sortInputError(err error) string {
	path := ""
	if readErr, ok := errors.AsType[sortReadError](err); ok {
		path = readErr.path
	}
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("sort: %s: No such file or directory", path)
	}
	return fmt.Sprintf("sort: %s: %v", path, err)
}

func writeSortDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(2)
}
