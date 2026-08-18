package runtime

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// `$'...'` -- ANSI-C quoting.
//
// The one way to write a tab or an escape character as an argument without a
// second program. Without it `printf '\033[1m'` has to be spelled through printf's
// own format, and `IFS=$'\n'` -- the standard way to make a loop iterate over lines
// rather than over words -- cannot be written at all. It used to expand to the
// dollar and the text: `$'a\tb'` gave the six characters `$a\tb`.
//
// The result is a *quoted* string: `$'a b'` is one word, measured against bash. So
// the decoded text goes in as a single-quoted part, which is what makes it survive
// field splitting and pathname expansion.

// decodeAnsiQuote reads a `$'...'` string starting at the `$`, and returns the
// decoded text with the number of input bytes consumed.
//
// The escapes are bash's, which are C's plus two: `\e` for escape, because nobody
// wants to write `\033` for a colour code, and `\cX` for a control character.
func decodeAnsiQuote(input string) (string, int, bool) {
	if !strings.HasPrefix(input, "$'") {
		return "", 0, false
	}
	var out strings.Builder
	index := 2
	for index < len(input) {
		char := input[index]
		if char == '\'' {
			return out.String(), index + 1, true
		}
		if char != '\\' || index+1 >= len(input) {
			out.WriteByte(char)
			index++
			continue
		}
		text, width := decodeAnsiEscape(input[index:])
		out.WriteString(text)
		index += width
	}
	// Unterminated. Reported as not an ANSI quote at all, so the caller's ordinary
	// unterminated-quote diagnostic is what the reader gets -- it is the same
	// mistake and there is no reason to have two words for it.
	return "", 0, false
}

// decodeAnsiEscape reads one backslash escape, returning its text and the bytes it
// took. An escape that is not one of these keeps the backslash, which is what bash
// does: `\q` is a backslash and a q.
func decodeAnsiEscape(input string) (string, int) {
	switch input[1] {
	case 'a':
		return "\a", 2
	case 'b':
		return "\b", 2
	case 'e', 'E':
		// bash's extension, and the reason this feature earns its keep: `$'\e[1m'`
		// against `printf '\033[1m'`.
		return "\x1b", 2
	case 'f':
		return "\f", 2
	case 'n':
		return "\n", 2
	case 'r':
		return "\r", 2
	case 't':
		return "\t", 2
	case 'v':
		return "\v", 2
	case '\\':
		return `\`, 2
	case '\'':
		return "'", 2
	case '"':
		return `"`, 2
	case '?':
		return "?", 2
	case 'x':
		return decodeAnsiNumber(input, 2, 16, 2)
	case 'u':
		return decodeAnsiRune(input, 4)
	case 'U':
		return decodeAnsiRune(input, 8)
	case 'c':
		return decodeAnsiControl(input)
	}
	if input[1] >= '0' && input[1] <= '7' {
		// Octal, up to three digits, and the leading zero is optional -- `\101` is
		// A and so is `\0101`.
		start := 1
		if input[1] == '0' && len(input) > 2 {
			start = 2
		}
		return decodeAnsiNumber(input, start, 8, 3)
	}
	return input[:2], 2
}

// decodeAnsiNumber reads up to limit digits in the given base, starting at offset.
func decodeAnsiNumber(input string, offset, base, limit int) (string, int) {
	end := offset
	for end < len(input) && end-offset < limit && isDigitInBase(input[end], base) {
		end++
	}
	if end == offset {
		// `\x` with no digits after it is the two characters, which is what bash
		// gives rather than an error.
		return input[:2], 2
	}
	value, err := strconv.ParseUint(input[offset:end], base, 32)
	if err != nil {
		return input[:2], 2
	}
	// A byte, not a rune: `\xff` is one byte in bash, not the two bytes of U+00FF.
	// Writing it as a rune would silently change the length of the string.
	return string([]byte{byte(value)}), end
}

// decodeAnsiRune reads \uHHHH or \UHHHHHHHH, which are code points rather than
// bytes.
func decodeAnsiRune(input string, digits int) (string, int) {
	end := 2
	for end < len(input) && end-2 < digits && isDigitInBase(input[end], 16) {
		end++
	}
	if end == 2 {
		return input[:2], 2
	}
	value, err := strconv.ParseUint(input[2:end], 16, 32)
	if err != nil || !utf8.ValidRune(rune(value)) {
		return input[:2], 2
	}
	return string(rune(value)), end
}

// decodeAnsiControl reads `\cX`, the control character X stands for: `\cA` is 1 and
// `\c[` is escape.
func decodeAnsiControl(input string) (string, int) {
	if len(input) < 3 {
		return input[:2], 2
	}
	char := input[2]
	if char >= 'a' && char <= 'z' {
		char -= 'a' - 'A'
	}
	return string([]byte{char & 0x1f}), 3
}

func isDigitInBase(char byte, base int) bool {
	switch {
	case char >= '0' && char <= '9':
		return int(char-'0') < base
	case base == 16 && char >= 'a' && char <= 'f', base == 16 && char >= 'A' && char <= 'F':
		return true
	}
	return false
}
