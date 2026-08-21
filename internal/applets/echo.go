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
		if value, width, ok := numericEscape(text[index:], true); ok {
			out.WriteByte(value)
			index += width - 1
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

// numericEscape reads `\NNN` and `\xHH`, reporting the byte and how much of rest it consumed.
//
// rest begins at the character *after* the backslash. Both forms exist in busybox-w32's
// bb_process_escape_sequence, which is the reference here, and both were missing: `printf '\x41'`
// and `echo -e '\x41'` printed the four characters back rather than an `A`, and bare octal did the
// same. Measured against busybox before and after -- `\x41\x42` is AB, `\101\102` is AB.
//
// leadingZero picks between the two octal forms, and they differ on purpose rather than by anyone's
// mistake. XSI specifies `\0NNN` for echo and POSIX specifies `\NNN` for printf's format, so
// busybox-w32 implements both and the two disagree about the same characters. Measured:
//
//	busybox echo -e '\0101'  ->  A     the zero is a marker, 101 is the value
//	busybox printf  '\0101'  ->  \b1   three digits is 010, and the 1 is literal
//
// Unifying them broke echo, and the test that caught it was already there and already right. The
// printf test beside it was not: it asserted echo's answer for printf, so it had been pinning a
// divergence from the reference since the day it was written.
//
// Hex takes at most two digits, so `\xABC` is 0xAB then a literal `C`, and a form with no digits
// at all (`\x`, `\xg`) is not an escape -- the caller leaves both bytes as they were written.
func numericEscape(rest string, leadingZero bool) (byte, int, bool) {
	if rest == "" {
		return 0, 0, false
	}
	if leadingZero && rest[0] == '0' {
		// XSI echo's form: the zero is a marker, and up to three digits follow it.
		value, digits := octalDigits(rest[1:], 3)
		if digits == 0 {
			return 0, 1, true
		}
		return value, digits + 1, true
	}
	if rest[0] == 'x' {
		digits := 0
		for digits < 2 && digits+1 < len(rest)+0 && isHexDigit(rest[digits+1]) {
			digits++
		}
		if digits == 0 {
			return 0, 0, false
		}
		value, err := strconv.ParseUint(rest[1:1+digits], 16, 16)
		if err != nil {
			return 0, 0, false
		}
		return byte(value), digits + 1, true
	}
	// Both forms in echo, which is the last thing measurement corrected here: busybox accepts
	// the XSI `\0NNN` *and* a bare `\NNN`, so `echo -e '\101'` is an A as well. Only the
	// leading-zero reading belongs to echo alone, and it is taken above.
	value, digits := octalDigits(rest, 3)
	if digits == 0 {
		return 0, 0, false
	}
	return value, digits, true
}

// octalDigits reads at most limit octal digits and reports how many it used.
func octalDigits(rest string, limit int) (byte, int) {
	digits := 0
	for digits < limit && digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '7' {
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

func isHexDigit(char byte) bool {
	switch {
	case char >= '0' && char <= '9':
		return true
	case char >= 'a' && char <= 'f':
		return true
	case char >= 'A' && char <= 'F':
		return true
	}
	return false
}
