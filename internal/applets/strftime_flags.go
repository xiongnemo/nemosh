package applets

import (
	"strings"
	"unicode"
)

// glibc's flags and widths, which GNU's date takes and busybox's takes on Linux, its strftime
// being glibc's: `%-d` is the day unpadded, `%_d` padded with a space, `%0e` with a zero, `%^a`
// upper case and `%#Z` the other case, `%10Y` ten wide, and an E or O before the conversion
// asks for an alternative this C locale does not have. Each was "unsupported format: %-".
// busybox-w32's strftime is the MSVC runtime's, which writes nothing at all for them.

// strftimeNumeric are the conversions that are numbers, whose padding the flags change; e, k
// and l are padded with spaces of their own.
const strftimeNumeric = "CdegGHIjklmMsSuUVwWyYq"

// strftimeModifiers reads the flags, the width and an E or O after a %, and reports how many
// bytes they took.
func strftimeModifiers(rest string) (flags string, width int, used int) {
	for used < len(rest) && strings.IndexByte("-_0^#", rest[used]) >= 0 {
		used++
	}
	flags = rest[:used]
	for used < len(rest) && rest[used] >= '0' && rest[used] <= '9' {
		width = width*10 + int(rest[used]-'0')
		used++
	}
	if used < len(rest) && (rest[used] == 'E' || rest[used] == 'O') {
		used++
	}
	return flags, width, used
}

// strftimePad applies the flags and the width to a rendered conversion.
func strftimePad(text string, verb byte, flags string, width int) string {
	pad := byte(0)
	for index := 0; index < len(flags); index++ {
		switch flags[index] {
		case '-', '_', '0':
			pad = flags[index]
		case '^':
			text = strings.ToUpper(text)
		case '#':
			if verb == 'p' || verb == 'Z' {
				text = strings.ToLower(text)
			} else {
				text = strings.ToUpper(text)
			}
		}
	}
	if strings.IndexByte(strftimeNumeric, verb) < 0 {
		// A word or a composite is padded on the left to the width, with spaces unless a 0
		// says zeros, and not at all under -.
		if pad == '-' || len(text) >= width {
			return text
		}
		fill := " "
		if pad == '0' {
			fill = "0"
		}
		return strings.Repeat(fill, width-len(text)) + text
	}
	natural := len(text)
	sign, digits := "", strings.TrimLeftFunc(text, unicode.IsSpace)
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	if pad != 0 {
		digits = strings.TrimLeft(digits, "0")
		if digits == "" {
			digits = "0"
		}
	}
	if pad == '-' {
		return sign + digits
	}
	if width == 0 {
		width = natural
	}
	fill := pad
	if fill == 0 {
		fill = '0'
		if strings.IndexByte("ekl", verb) >= 0 && strings.HasPrefix(text, " ") {
			fill = ' '
		}
	}
	if fill == '_' {
		fill = ' '
	}
	if missing := width - len(sign) - len(digits); missing > 0 {
		if fill == ' ' {
			return strings.Repeat(" ", missing) + sign + digits
		}
		return sign + strings.Repeat("0", missing) + digits
	}
	return sign + digits
}
