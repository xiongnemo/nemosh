package applets

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
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
			// A usage error, 2, as busybox's printf answers it.
			return ExitStatusMessage(2, missingOperand())
		}
		format, operands := args[0], args[1:]
		invalid := false
		for pass := 0; ; pass++ {
			consumed, failed, err := writePrintfPass(stdout, stderr, format, operands)
			invalid = invalid || failed
			if errors.Is(err, errPrintfStop) {
				break
			}
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

// errPrintfNumber is an operand a numeric conversion could not read. It is reported in
// busybox's words, multiconvert's, and written as zero, which is busybox's answer -- bash
// writes the digits it managed to read, so `12abc` is 12 there and 0 here.
type errPrintfNumber struct{ operand string }

func (e errPrintfNumber) Error() string { return fmt.Sprintf("invalid number '%s'", e.operand) }

// errPrintfStop is \c, in the format or in a %b operand: output ends there, and no operand
// after it is used -- `printf '%s\c' x y` is x, as in busybox, where it went on to y.
var errPrintfStop = errors.New("stopped by \\c")

// writePrintfPass walks the format once and reports how many operands it used, and whether
// one of them was not the number its conversion wanted.
func writePrintfPass(out, diagnostics io.Writer, format string, operands []string) (int, bool, error) {
	used, failed := 0, false
	next := func() (string, bool) {
		if used < len(operands) {
			value := operands[used]
			used++
			return value, true
		}
		used++
		return "", false
	}
	var text strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] == '\\' {
			replacement, width, stop := printfEscape(format[index+1:])
			text.WriteString(replacement)
			index += width
			if stop {
				return used, failed, writePrintfStop(out, text.String())
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
		if spec, layout, width, ok := printfTimeSpecification(format[index:]); ok {
			operand, _ := next()
			rendered, err := renderPrintfTime(spec, layout, operand)
			if err != nil {
				fmt.Fprintf(diagnostics, "printf: %v\n", err)
				failed = true
			}
			text.WriteString(rendered)
			index += width - 1
			continue
		}
		spec, verb, width := printfSpecification(format[index:])
		if verb == 0 {
			text.WriteByte('%')
			continue
		}
		spec, err := resolvePrintfStars(spec, next)
		rendered, renderErr := renderPrintfConversion(spec, verb, next)
		if err == nil {
			err = renderErr
		}
		if errors.Is(err, errPrintfStop) {
			text.WriteString(rendered)
			return used, failed, writePrintfStop(out, text.String())
		}
		var number errPrintfNumber
		if errors.As(err, &number) {
			// Said now, in order with the output, and processing goes on.
			fmt.Fprintf(diagnostics, "printf: %v\n", err)
			failed, err = true, nil
		}
		if err != nil {
			// What came before the conversion is written first, as busybox and bash
			// write it; it was dropped with the rest.
			if _, writeErr := io.WriteString(out, text.String()); writeErr != nil {
				return used, failed, writeErr
			}
			return used, failed, err
		}
		text.WriteString(rendered)
		index += width - 1
	}
	_, err := io.WriteString(out, text.String())
	return used, failed, err
}

// writePrintfStop writes what came before a \c, and says that it came.
func writePrintfStop(out io.Writer, text string) error {
	if _, err := io.WriteString(out, text); err != nil {
		return err
	}
	return errPrintfStop
}

// printfSpecification reads `%[flags][width][.precision]verb` and reports the
// specification, its verb, and how many bytes it occupies.
func printfSpecification(rest string) (string, byte, int) {
	index := 1
	for index < len(rest) && strings.IndexByte("-+ #0", rest[index]) >= 0 {
		index++
	}
	index = printfNumberOrStar(rest, index)
	if index < len(rest) && rest[index] == '.' {
		index = printfNumberOrStar(rest, index+1)
	}
	if index >= len(rest) {
		return "", 0, 0
	}
	return rest[:index], rest[index], index + 1
}

// printfNext hands a conversion its operand, and says whether there was one: one that is
// missing is zero to a number, where one given empty is a number neither reference reads.
type printfNext func() (string, bool)

func renderPrintfConversion(spec string, verb byte, next printfNext) (string, error) {
	if strings.IndexByte("diouxXeEfFgGcsbq", verb) < 0 {
		return "", fmt.Errorf("invalid conversion specification %%%c", verb)
	}
	operand, given := next()
	switch verb {
	case 'd', 'i':
		// The zero an unreadable operand stands for is rendered with the operand's own
		// width and flags, and the error goes back beside it for the caller to report.
		value, err := printfSigned(operand, given)
		return fmt.Sprintf(spec+"d", value), err
	case 'o', 'x', 'X', 'u':
		value, err := printfUnsigned(operand, given)
		if verb == 'u' {
			return fmt.Sprintf(spec+"d", value), err
		}
		if value == 0 {
			// C's # puts no 0x on a zero, where Go's writes 0x0.
			spec = strings.ReplaceAll(spec, "#", "")
		}
		return fmt.Sprintf(spec+string(verb), value), err
	case 'e', 'E', 'f', 'F', 'g', 'G':
		// C's conversions rather than Go's, see cFloat: %g has six digits unless the precision
		// says, where Go's has as many as read back, so `printf %g 123456789` was
		// 1.23456789e+08 where both references say 1.23457e+08; and inf is inf, not +Inf.
		layout := parseCSpec(strings.TrimPrefix(spec, "%"))
		if code, ok := printfCharacterCode(strings.TrimSpace(operand)); ok {
			return cFloat(layout, verb, float64(code)), nil
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(operand), 64)
		if err != nil && strings.TrimSpace(operand) != "" {
			return cFloat(layout, verb, 0), errPrintfNumber{operand: operand}
		}
		return cFloat(layout, verb, value), nil
	case 'c':
		if operand == "" {
			// The empty string's first character is its end, a NUL, which both references
			// write, padded to the width; this wrote nothing.
			return fmt.Sprintf(spec+"c", 0), nil
		}
		return fmt.Sprintf(spec+"s", operand[:1]), nil
	case 's':
		return fmt.Sprintf(spec+"s", operand), nil
	case 'b':
		// XSI's %b: the operand's own escape sequences are processed. A \c among them ends
		// all output, the rest of the format and every operand to come, as in busybox.
		expanded, stop := expandEchoEscapes(operand)
		if stop {
			return fmt.Sprintf(spec+"s", expanded), errPrintfStop
		}
		return fmt.Sprintf(spec+"s", expanded), nil
	case 'q':
		// bash's %q: quote the operand so the shell would read it back as itself.
		// The point of it is `eval` and generated scripts -- a file name with a
		// space or a quote in it survives being written into a command line.
		return fmt.Sprintf(spec+"s", shellquote.Backslash(operand)), nil
	default:
		return "", fmt.Errorf("invalid conversion specification %%%c", verb)
	}
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
	if value, width, ok := unicodeEscape(rest); ok {
		return value, width, false
	}
	if value, width, ok := numericEscape(rest, false); ok {
		return string([]byte{value}), width, false
	}
	if replacement, ok := simpleEscape(rest[0]); ok {
		return string([]byte{replacement}), 1, false
	}
	return `\` + string(rest[0]), 1, false
}
