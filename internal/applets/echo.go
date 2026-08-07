package applets

import (
	"io"
	"strconv"
	"strings"
)

// newEchoApplet implements busybox's FEATURE_FANCY_ECHO rules (coreutils/echo.c):
// leading arguments made only of `n`, `e`, and `E` after the `-` are options,
// and the first argument that is not -- including a lone `-`, and including
// `-nz`, where one letter of the cluster is unknown -- ends option parsing and
// is echoed as written. So `echo -n` prints nothing and `echo -z` prints `-z`.
//
// Nemosh printed `-n abc` for `echo -n abc`, which is what an echo with no
// option handling at all does.
func newEchoApplet() Applet {
	return simpleApplet{name: "echo", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		newline, escapes, operands := echoOptions(args)
		text := strings.Join(operands, " ")
		if escapes {
			var cancelled bool
			text, cancelled = expandEchoEscapes(text)
			newline = newline && !cancelled
		}
		if newline {
			text += "\n"
		}
		_, err := io.WriteString(stdout, text)
		return err
	}}
}

func echoOptions(args []string) (bool, bool, []string) {
	newline, escapes := true, false
	for index, arg := range args {
		if len(arg) < 2 || arg[0] != '-' {
			return newline, escapes, args[index:]
		}
		// Provisional: a cluster is only accepted once every letter in it is
		// known, so `-nz` leaves both untouched and is echoed instead.
		candidateNewline, candidateEscapes := newline, escapes
		for position := 1; position < len(arg); position++ {
			switch arg[position] {
			case 'n':
				candidateNewline = false
			case 'e':
				candidateEscapes = true
			case 'E':
				candidateEscapes = false
			default:
				return newline, escapes, args[index:]
			}
		}
		newline, escapes = candidateNewline, candidateEscapes
	}
	return newline, escapes, nil
}

// expandEchoEscapes handles the sequences XSI echo lists, plus the `\e` busybox
// adds. The second result reports `\c`, which cancels the trailing newline and
// everything after it.
func expandEchoEscapes(text string) (string, bool) {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '\\' || index+1 >= len(text) {
			out.WriteByte(text[index])
			continue
		}
		index++
		if text[index] == 'c' {
			return out.String(), true
		}
		if text[index] == '0' {
			value, width := octalEscape(text[index+1:])
			out.WriteByte(value)
			index += width
			continue
		}
		if replacement, ok := simpleEscape(text[index]); ok {
			out.WriteByte(replacement)
			continue
		}
		// An unknown sequence keeps both bytes, which is what busybox's
		// bb_process_escape_sequence leaves behind.
		out.WriteByte('\\')
		out.WriteByte(text[index])
	}
	return out.String(), false
}

func simpleEscape(char byte) (byte, bool) {
	switch char {
	case 'a':
		return 0x07, true
	case 'b':
		return 0x08, true
	case 'e':
		return 0x1b, true
	case 'f':
		return 0x0c, true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return 0x0b, true
	case '\\':
		return '\\', true
	}
	return 0, false
}

// octalEscape reads the digits after the `\0` SUSv3 requires, at most three of
// them, and reports how many it consumed.
func octalEscape(rest string) (byte, int) {
	digits := 0
	for digits < 3 && digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '7' {
		digits++
	}
	if digits == 0 {
		return 0, 0
	}
	value, err := strconv.ParseUint(rest[:digits], 8, 16)
	if err != nil {
		return 0, 0
	}
	return byte(value), digits
}
