package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

func newCutApplet() Applet {
	return cutApplet{}
}

type cutApplet struct{}

func (cutApplet) Name() string {
	return "cut"
}

func (cutApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	options, err := parseCutArgs(args)
	if err != nil {
		return writeCutDiagnostic(stderr, err.Error())
	}
	// runCutInputs reports each unreadable operand as it goes, so what comes
	// back is already either a context error or the final exit status.
	return runCutInputs(ctx, ProcessViewFromContext(ctx), options, stdin, stdout, stderr)
}

type cutMode int

const (
	cutModeNone cutMode = iota
	cutModeBytes
	cutModeChars
	cutModeFields
)

type cutOptions struct {
	mode     cutMode
	ranges   []cutRange
	delim    byte
	hasDelim bool
	suppress bool
	operands []string
}

func parseCutArgs(args []string) (cutOptions, error) {
	options := cutOptions{delim: '\t'}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			options.operands = append(options.operands, args[index+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			options.operands = append(options.operands, arg)
			continue
		}
		var err error
		index, err = parseCutOption(args, index, &options)
		if err != nil {
			return cutOptions{}, err
		}
	}
	if options.mode == cutModeNone {
		return cutOptions{}, errors.New("cut: expected a list of bytes, characters, or fields")
	}
	if options.hasDelim && options.mode != cutModeFields {
		return cutOptions{}, errors.New("cut: -d DELIM requires -f")
	}
	if options.suppress && options.mode != cutModeFields {
		return cutOptions{}, errors.New("cut: -s requires -f")
	}
	return options, nil
}

func parseCutOption(args []string, index int, options *cutOptions) (int, error) {
	arg := args[index]
	for offset := 1; offset < len(arg); offset++ {
		flag := arg[offset]
		switch flag {
		case 'b', 'c', 'f':
			value, nextIndex, err := cutOptionArgument(args, index, offset)
			if err != nil {
				return index, err
			}
			if err := setCutMode(options, flag, value); err != nil {
				return index, err
			}
			return nextIndex, nil
		case 'd':
			value, nextIndex, err := cutOptionArgument(args, index, offset)
			if err != nil {
				return index, err
			}
			if value == "" {
				return index, errors.New("cut: empty delimiter")
			}
			options.delim = value[0]
			options.hasDelim = true
			return nextIndex, nil
		case 's':
			options.suppress = true
		case 'n':
			continue
		default:
			return index, fmt.Errorf("cut: invalid option -- %c", flag)
		}
	}
	return index, nil
}

func cutOptionArgument(args []string, index int, offset int) (string, int, error) {
	arg := args[index]
	if offset+1 < len(arg) {
		return arg[offset+1:], index, nil
	}
	if index+1 >= len(args) || args[index+1] == "--" {
		return "", index, fmt.Errorf("cut: option requires an argument -- %c", arg[offset])
	}
	return args[index+1], index + 1, nil
}

func setCutMode(options *cutOptions, flag byte, list string) error {
	if options.mode != cutModeNone {
		return errors.New("cut: options -b, -c, and -f are mutually exclusive")
	}
	ranges, err := parseCutRanges(list)
	if err != nil {
		return err
	}
	options.ranges = ranges
	switch flag {
	case 'b':
		options.mode = cutModeBytes
	case 'c':
		options.mode = cutModeChars
	case 'f':
		options.mode = cutModeFields
	}
	return nil
}

func cutReader(input io.Reader, stdout io.Writer, options cutOptions) error {
	reader := bufio.NewReader(input)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if err := writeCutLine(stdout, line, options); err != nil {
				return err
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

func writeCutLine(stdout io.Writer, line string, options cutOptions) error {
	if options.mode == cutModeFields {
		selected, ok := selectCutFields(line, options)
		if !ok {
			return nil
		}
		_, err := fmt.Fprintln(stdout, selected)
		return err
	}
	_, err := fmt.Fprintln(stdout, selectCutBytes(line, options.ranges))
	return err
}

func selectCutBytes(line string, ranges []cutRange) string {
	var builder strings.Builder
	for index := range len(line) {
		position := index + 1
		if cutRangeContains(ranges, position) {
			builder.WriteByte(line[index])
		}
	}
	return builder.String()
}

func selectCutFields(line string, options cutOptions) (string, bool) {
	delim := string([]byte{options.delim})
	if !strings.Contains(line, delim) {
		return line, !options.suppress
	}
	fields := strings.Split(line, delim)
	selected := make([]string, 0, len(fields))
	for index, field := range fields {
		position := index + 1
		if cutRangeContains(options.ranges, position) {
			selected = append(selected, field)
		}
	}
	return strings.Join(selected, delim), true
}

// cut never raises xfunc_error_retval, so both its usage deaths and its
// per-operand `retval = EXIT_FAILURE` land on the libbb default of 1
// (libbb/default_error_retval.c:16). Only sort asks for 2.
func writeCutDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ExitStatus(1)
}
