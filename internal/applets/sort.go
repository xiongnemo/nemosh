package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
		return writeSortDiagnostic(stderr, inputDiagnostic("sort", err))
	}
	sortTextLines(lines, options)
	if options.fields.unique {
		// After sorting, because -u removes duplicates *by the comparison key*
		// rather than as text: `sort -uf` on `B b a` answers `a B`.
		lines = uniqueSorted(lines, options.fields, options.numeric)
	}
	return writeSortLines(stdout, lines)
}

type sortOptions struct {
	numeric bool
	reverse bool
	fields  sortFields
	paths   []string
}

func parseSortArgs(args []string) (sortOptions, error) {
	var options sortOptions
	skip := -1
	for index := range len(args) {
		if index <= skip {
			continue
		}
		arg := args[index]
		if arg == "--" {
			options.paths = append(options.paths, args[index+1:]...)
			return options, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			options.paths = append(options.paths, arg)
			continue
		}
		// A long option is one word. Read letter by letter it reports the `-` it
		// begins with, so `--nonsense` came back as `invalid option -- -`, which
		// names nothing the user wrote. sort has no long options to accept; it
		// only has to say which one it is refusing.
		if strings.HasPrefix(arg, "--") {
			return sortOptions{}, fmt.Errorf("sort: unrecognized option %s", arg)
		}
		// -k and -t take a value, attached or as the next word, so they are
		// settled before the remaining letters are walked.
		letters, consumed, err := sortValueOption(args, index, &options.fields)
		if err != nil {
			return sortOptions{}, err
		}
		if consumed > 0 {
			skip = index + consumed - 1
			arg = "-" + letters
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'n':
				options.numeric = true
			case 'r':
				options.reverse = true
			case 'u':
				options.fields.unique = true
			case 'f':
				options.fields.foldCase = true
			case 'b':
				options.fields.ignoreBlanks = true
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
		input, err := OpenProcessOperand(ctx, view, path, stdin)
		if err != nil {
			return nil, inputFailure(path, err)
		}
		fileLines, readErr := readSortLines(input)
		closeErr := input.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, inputFailure(path, err)
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
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
	return compareSortKeys(left, right, options.fields, options.numeric)
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

// sort is the one reader entitled to 2: sort_main sets `xfunc_error_retval = 2`
// before parsing options (coreutils/sort.c:468), so every bb_show_usage and
// every xfopen_stdin death after it carries that status.
func writeSortDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(2)
}
