package applets

import "strings"

// unescapeTrSet reads the backslash escapes tr defines. A set is nearly always
// single-quoted, so the shell hands `\r` over as two characters and tr is the
// one that has to know what they mean.
//
// Those are busybox's: \n \r \t, \a \b \f \v, \\, and an octal number of up to three digits
// and at most 0377, so `\400` is a space and then a 0. The octal one is how a NUL is named:
// `tr '\0' '\n'` makes lines of `find -print0`'s list. Without it `\0` was a 0, and that did
// nothing.
func unescapeTrSet(set string) string {
	var out strings.Builder
	for index := 0; index < len(set); index++ {
		if set[index] != '\\' || index+1 == len(set) {
			out.WriteByte(set[index])
			continue
		}
		index++
		if value, digits := trOctal(set[index:]); digits > 0 {
			out.WriteByte(value)
			index += digits - 1
			continue
		}
		out.WriteByte(trEscape(set[index]))
	}
	return out.String()
}

// trOctal is the octal number text begins with, and how many digits it took.
func trOctal(text string) (byte, int) {
	value, digits := 0, 0
	for digits < 3 && digits < len(text) && text[digits] >= '0' && text[digits] <= '7' {
		next := value*8 + int(text[digits]-'0')
		if next > 0377 {
			break
		}
		value, digits = next, digits+1
	}
	return byte(value), digits
}

// trEscape is the character a backslash and letter stand for; any other character stands for
// itself.
func trEscape(letter byte) byte {
	switch letter {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'v':
		return '\v'
	}
	return letter
}
