package applets

import (
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
func newPrintfApplet() Applet {
	return simpleApplet{name: "printf", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return missingOperand()
		}
		format, operands := args[0], args[1:]
		for pass := 0; ; pass++ {
			consumed, err := writePrintfPass(stdout, format, operands)
			if err != nil {
				return err
			}
			// A pass that consumes nothing would repeat forever; one pass is
			// always run so a format with no conversions still prints.
			if consumed == 0 || consumed >= len(operands) {
				return nil
			}
			operands = operands[consumed:]
		}
	}}
}

// writePrintfPass walks the format once and reports how many operands it used.
func writePrintfPass(out io.Writer, format string, operands []string) (int, error) {
	used := 0
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
				return used, err
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
		if err != nil {
			return used, err
		}
		text.WriteString(rendered)
		index += width - 1
	}
	_, err := io.WriteString(out, text.String())
	return used, err
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
		value, err := printfInteger(next())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(spec+"d", value), nil
	case 'o', 'x', 'X', 'u':
		value, err := printfInteger(next())
		if err != nil {
			return "", err
		}
		if verb == 'u' {
			return fmt.Sprintf(spec+"d", value), nil
		}
		return fmt.Sprintf(spec+string(verb), value), nil
	case 'e', 'E', 'f', 'F', 'g', 'G':
		value, err := strconv.ParseFloat(strings.TrimSpace(next()), 64)
		if err != nil {
			return "", fmt.Errorf("invalid number")
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
	default:
		return "", fmt.Errorf("invalid conversion specification %%%c", verb)
	}
}

// An operand that is not a number is zero with a diagnostic in POSIX; busybox
// and the shells all reject it, and so does this.
func printfInteger(operand string) (int64, error) {
	trimmed := strings.TrimSpace(operand)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmed, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: expected a numeric value", operand)
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
	if rest[0] == '0' {
		value, digits := octalEscape(rest[1:])
		if digits > 0 {
			return string([]byte{value}), digits + 1, false
		}
		return "\x00", 1, false
	}
	if replacement, ok := simpleEscape(rest[0]); ok {
		return string([]byte{replacement}), 1, false
	}
	return `\` + string(rest[0]), 1, false
}
