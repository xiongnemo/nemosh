package applets

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// printf implements the POSIX utility rather than handing the format to Go's
// Fprintf. Every operand arrived as a Go string before, so `printf '%d\n' 42`
// printed `%!d(string=42)` and `printf '%5.2f\n' 3.14159` printed
// `%!f(string=   3.)` -- garbage, quietly, with status 0.
//
// The conversions are the ones POSIX lists, plus the `%b` of XSI, and the
// format is reused from the start while operands remain, which is what makes
// `printf '%s\n' a b c` print three lines.
//
// A leading `--` ends the options, as in both references; it was taken for the format and
// printed. And an operand that is not a number is POSIX's case exactly: a diagnostic, zero
// written in its place, the rest processed, and a status that is not zero. This stopped at
// the first one and wrote nothing after it.
func newPrintfApplet() Applet {
	return simpleApplet{name: "printf", run: func(args []string, _ io.Reader, stdout, stderr io.Writer) error {
		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
		if len(args) == 0 {
			return missingOperand()
		}
		format, operands := args[0], args[1:]
		invalid := false
		for pass := 0; ; pass++ {
			consumed, failed, err := writePrintfPass(stdout, stderr, format, operands)
			invalid = invalid || failed
			if err != nil {
				return err
			}
			// A pass that consumes nothing would repeat forever; one pass is
			// always run so a format with no conversions still prints.
			if consumed == 0 || consumed >= len(operands) {
				break
			}
			operands = operands[consumed:]
		}
		if invalid {
			return ExitStatus(1)
		}
		return nil
	}}
}

// errPrintfNumber is an operand a numeric conversion could not read. It is reported and
// written as zero, which is busybox's answer -- bash writes the digits it managed to read,
// so `12abc` is 12 there and 0 here.
type errPrintfNumber struct{ operand string }

func (e errPrintfNumber) Error() string { return fmt.Sprintf("%s: invalid number", e.operand) }

// writePrintfPass walks the format once and reports how many operands it used, and whether
// one of them was not the number its conversion wanted.
func writePrintfPass(out, diagnostics io.Writer, format string, operands []string) (int, bool, error) {
	used, failed := 0, false
	next := func() string {
		if used < len(operands) {
			value := operands[used]
			used++
			return value
		}
		used++
		return ""
	}
	var text strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] == '\\' {
			replacement, width, stop := printfEscape(format[index+1:])
			text.WriteString(replacement)
			index += width
			if stop {
				_, err := io.WriteString(out, text.String())
				return used, failed, err
			}
			continue
		}
		if format[index] != '%' {
			text.WriteByte(format[index])
			continue
		}
		if index+1 < len(format) && format[index+1] == '%' {
			text.WriteByte('%')
			index++
			continue
		}
		spec, verb, width := printfSpecification(format[index:])
		if verb == 0 {
			text.WriteByte('%')
			continue
		}
		rendered, err := renderPrintfConversion(spec, verb, next)
		var number errPrintfNumber
		if errors.As(err, &number) {
			// Said now, in order with the output, and processing goes on.
			fmt.Fprintf(diagnostics, "printf: %v\n", err)
			failed, err = true, nil
		}
		if err != nil {
			return used, failed, err
		}
		text.WriteString(rendered)
		index += width - 1
	}
	_, err := io.WriteString(out, text.String())
	return used, failed, err
}

// printfSpecification reads `%[flags][width][.precision]verb` and reports the
// specification, its verb, and how many bytes it occupies.
func printfSpecification(rest string) (string, byte, int) {
	index := 1
	for index < len(rest) && strings.IndexByte("-+ #0", rest[index]) >= 0 {
		index++
	}
	for index < len(rest) && rest[index] >= '0' && rest[index] <= '9' {
		index++
	}
	if index < len(rest) && rest[index] == '.' {
		index++
		for index < len(rest) && rest[index] >= '0' && rest[index] <= '9' {
			index++
		}
	}
	if index >= len(rest) {
		return "", 0, 0
	}
	return rest[:index], rest[index], index + 1
}

func renderPrintfConversion(spec string, verb byte, next func() string) (string, error) {
	switch verb {
	case 'd', 'i':
		// The zero an unreadable operand stands for is rendered with the operand's own
		// width and flags, and the error goes back beside it for the caller to report.
		value, err := printfInteger(next())
		return fmt.Sprintf(spec+"d", value), err
	case 'o', 'x', 'X', 'u':
		value, err := printfInteger(next())
		if verb == 'u' {
			return fmt.Sprintf(spec+"d", value), err
		}
		return fmt.Sprintf(spec+string(verb), value), err
	case 'e', 'E', 'f', 'F', 'g', 'G':
		operand := next()
		value, err := strconv.ParseFloat(strings.TrimSpace(operand), 64)
		if err != nil && strings.TrimSpace(operand) != "" {
			return fmt.Sprintf(spec+string(verb), 0.0), errPrintfNumber{operand: operand}
		}
		return fmt.Sprintf(spec+string(verb), value), nil
	case 'c':
		operand := next()
		if operand == "" {
			return "", nil
		}
		return fmt.Sprintf(spec+"s", operand[:1]), nil
	case 's':
		return fmt.Sprintf(spec+"s", next()), nil
	case 'b':
		// XSI's %b: the operand's own escape sequences are processed.
		expanded, _ := expandEchoEscapes(next())
		return fmt.Sprintf(spec+"s", expanded), nil
	case 'q':
		// bash's %q: quote the operand so the shell would read it back as itself.
		// The point of it is `eval` and generated scripts -- a file name with a
		// space or a quote in it survives being written into a command line.
		return fmt.Sprintf(spec+"s", shellQuote(next())), nil
	default:
		return "", fmt.Errorf("invalid conversion specification %%%c", verb)
	}
}

// An operand that is not a number is zero with a diagnostic, in POSIX and in both
// references -- measured, and not the rejection this comment used to claim.
func printfInteger(operand string) (int64, error) {
	trimmed := strings.TrimSpace(operand)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmed, 0, 64)
	if err != nil {
		return 0, errPrintfNumber{operand: operand}
	}
	return value, nil
}

// printfEscape reads the sequence after a backslash in the format and reports
// the replacement, how many extra bytes it used, and whether `\c` ended output.
func printfEscape(rest string) (string, int, bool) {
	if rest == "" {
		return `\`, 0, false
	}
	if rest[0] == 'c' {
		return "", 1, true
	}
	if value, width, ok := numericEscape(rest, false); ok {
		return string([]byte{value}), width, false
	}
	if replacement, ok := simpleEscape(rest[0]); ok {
		return string([]byte{replacement}), 1, false
	}
	return `\` + string(rest[0]), 1, false
}
