package applets

import (
	"fmt"
	"strconv"
)

// The escapes a quoted awk string may contain.
//
// Separate from the scanner because the same table is wanted twice: here for a string
// literal, and later for `printf`'s format, where the shell's own `printfEscape`
// (printf.go) answers a slightly different question and cannot be shared.
//
// **An unknown escape is the character itself**, with the backslash dropped -- `"a\qb"`
// is `aqb`. Measured: busybox does exactly that, and gawk warns and then does the same.
// POSIX leaves it undefined, so the primary reference decides.

// awkDecodeEscape reads one escape, given the text *after* the backslash, and answers
// what it stands for and how many bytes it used.
func awkDecodeEscape(rest string, line int) (string, int, error) {
	if rest == "" {
		return "", 0, fmt.Errorf("line %d: trailing backslash", line)
	}
	switch rest[0] {
	case 'n':
		return "\n", 1, nil
	case 't':
		return "\t", 1, nil
	case 'r':
		return "\r", 1, nil
	case '\\':
		return "\\", 1, nil
	case '"':
		return "\"", 1, nil
	case '/':
		// Legal in a string as well as in a regex, and both references accept it.
		return "/", 1, nil
	case 'a':
		return "\a", 1, nil
	case 'b':
		return "\b", 1, nil
	case 'f':
		return "\f", 1, nil
	case 'v':
		return "\v", 1, nil
	}
	// An octal escape of one to three digits: `"a\061b"` is `a1b`, measured.
	if rest[0] >= '0' && rest[0] <= '7' {
		width := 1
		for width < 3 && width < len(rest) && rest[width] >= '0' && rest[width] <= '7' {
			width++
		}
		value, err := strconv.ParseUint(rest[:width], 8, 16)
		if err != nil {
			return "", 0, fmt.Errorf("line %d: bad octal escape", line)
		}
		return string([]byte{byte(value)}), width, nil
	}
	return rest[:1], 1, nil
}
