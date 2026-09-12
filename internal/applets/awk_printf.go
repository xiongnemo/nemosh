package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// `printf` and `sprintf`.
//
// The shell's printf applet (printf.go) is not shared, and the reasons are worth writing
// down because sharing it looks obvious:
//
//   - **awk's operands are typed.** `printfInteger` parses a string with `ParseInt` base 0,
//     which accepts `0x1A`; awk does not -- `printf "%d", "0x1A"` is 0 -- and it rejects
//     `"3.9"`, where awk answers 3.
//   - **awk allows `*` for a width or a precision**, taken from an argument. The shell's
//     grammar has no such thing, so `printfSpecification` stops dead at the `*`. busybox
//     refuses `%*d` outright; gawk supports it and is followed.
//   - **The format is not reused.** `printf "%s\n", "a", "b"` prints one line and drops
//     `b`, where the shell's printf prints two. Extra arguments are ignored, and a missing
//     one is the empty string or zero -- POSIX and busybox, where gawk makes it fatal.
//
// One rule is shared by *not* being applied: a format's `\n` was already decoded when the
// string literal was lexed, so this does no escape processing at all. Both references
// confirm it -- `s = "a\\nb"; printf s` prints a literal `a\nb`.

func (in *awkInterp) builtinSprintf(node awkBuiltinExpr) (awkValue, error) {
	format, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	args, err := in.evalArguments(node.args[1:])
	if err != nil {
		return awkValue{}, err
	}
	text, err := in.sprintf(format, args)
	if err != nil {
		return awkValue{}, err
	}
	return awkStr(text), nil
}

// execPrintf is the statement, which is `sprintf` written straight to the output.
func (in *awkInterp) execPrintf(node awkPrintfStmt) error {
	args := awkUnwrapPrintList(node.args)
	if len(args) == 0 {
		return in.errorf("printf needs a format")
	}
	format, err := in.eval(args[0])
	if err != nil {
		return err
	}
	values, err := in.evalArguments(args[1:])
	if err != nil {
		return err
	}
	text, err := in.sprintf(format.str(in.convfmt()), values)
	if err != nil {
		return err
	}
	return in.writeTo(node.redirect, text)
}

func (in *awkInterp) evalArguments(args []awkExpr) ([]awkValue, error) {
	values := make([]awkValue, 0, len(args))
	for _, arg := range args {
		value, err := in.eval(arg)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// sprintf renders a format with awk's own conversions.
func (in *awkInterp) sprintf(format string, args []awkValue) (string, error) {
	used := 0
	next := func() awkValue {
		if used < len(args) {
			value := args[used]
			used++
			return value
		}
		// Running out is not an error: the remaining conversions see the uninitialised
		// value, which is both "" and 0, so `%s` prints nothing and `%d` prints 0.
		used++
		return awkValue{}
	}
	var out strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] != '%' {
			out.WriteByte(format[index])
			continue
		}
		if index+1 < len(format) && format[index+1] == '%' {
			out.WriteByte('%')
			index++
			continue
		}
		spec, verb, width := awkPrintfSpec(format[index:], next)
		if verb == 0 {
			// A `%` with nothing after it is itself, rather than a failure.
			out.WriteByte('%')
			continue
		}
		rendered, err := in.renderAwkConversion(spec, verb, next)
		if err != nil {
			return "", err
		}
		out.WriteString(rendered)
		index += width - 1
	}
	return out.String(), nil
}

// awkPrintfSpec reads `%[flags][width][.precision]verb`, resolving a `*` from the arguments
// as it goes, and answers a specification Go's own formatter can take.
func awkPrintfSpec(rest string, next func() awkValue) (string, byte, int) {
	var spec strings.Builder
	spec.WriteByte('%')
	index := 1
	for index < len(rest) && strings.IndexByte("-+ #0", rest[index]) >= 0 {
		spec.WriteByte(rest[index])
		index++
	}
	index = awkPrintfNumber(rest, index, &spec, next)
	if index < len(rest) && rest[index] == '.' {
		spec.WriteByte('.')
		index++
		index = awkPrintfNumber(rest, index, &spec, next)
	}
	if index >= len(rest) {
		return "", 0, 0
	}
	return spec.String(), rest[index], index + 1
}

// awkPrintfNumber copies a width or a precision, which is digits or a `*` standing for an
// argument.
//
// A `*` is written out as the digits it stood for, so a negative one becomes the `-` flag
// and the width that follows it -- which is what C does and what makes `printf "%*d", -5, 1`
// left-justify.
func awkPrintfNumber(rest string, index int, spec *strings.Builder, next func() awkValue) int {
	if index < len(rest) && rest[index] == '*' {
		spec.WriteString(strconv.FormatInt(awkToInt64(next().num()), 10))
		return index + 1
	}
	for index < len(rest) && rest[index] >= '0' && rest[index] <= '9' {
		spec.WriteByte(rest[index])
		index++
	}
	return index
}

func (in *awkInterp) renderAwkConversion(spec string, verb byte, next func() awkValue) (string, error) {
	switch verb {
	case 'd', 'i', 'u':
		// `%i` is `%d`, and `%u` is too: awk has one numeric type and no unsigned form,
		// and both references print a negative operand as negative.
		return fmt.Sprintf(spec+"d", awkToInt64(next().num())), nil
	case 'o', 'x', 'X':
		return fmt.Sprintf(spec+string(verb), awkToInt64(next().num())), nil
	case 'e', 'E', 'f', 'F', 'g', 'G':
		// Go writes a two-digit exponent, where busybox on Windows writes three
		// (`1.234500e+003`). That is the MSVC runtime rather than a rule; C99 and gawk
		// both say two.
		return fmt.Sprintf(spec+string(verb), next().num()), nil
	case 'c':
		return fmt.Sprintf(spec+"s", in.printfChar(next())), nil
	case 's':
		return fmt.Sprintf(spec+"s", next().str(in.convfmt())), nil
	}
	return "", in.errorf("%%%c is not a conversion awk knows", verb)
}

// printfChar is `%c`, which asks what kind of value it was handed.
//
// A **number** is a character code, so `printf "%c", 65` prints `A`; a string is its first
// character. A strnum counts as a number, which is what makes `echo 65 | awk '{printf "%c",
// $1}'` print `A` rather than `6` -- measured, and both references agree.
//
// The code becomes a **rune**: `printf "%c", 233` is `é` here, where the references in a C
// locale write the single byte 0xE9. That is awk_builtin.go's rune rule again, and it keeps
// the output valid UTF-8 rather than a byte no terminal can show.
func (in *awkInterp) printfChar(value awkValue) string {
	if value.comparesAsNumber() {
		return string(rune(awkToInt64(value.num())))
	}
	text := value.str(in.convfmt())
	if text == "" {
		// The first character of nothing is the NUL, which is what both references write.
		return "\x00"
	}
	return string([]rune(text)[0])
}

// awkUnwrapPrintList opens out `print (a, b)` and `printf("%s\n", x)`, where the whole
// argument list wears one pair of parentheses.
//
// A single argument that is an ordinary group stays one: `print (1 > 0)` is a comparison
// and depends on the parenthesis surviving.
func awkUnwrapPrintList(args []awkExpr) []awkExpr {
	if len(args) != 1 {
		return args
	}
	if list, ok := args[0].(awkGroupListExpr); ok {
		return list.items
	}
	return args
}
