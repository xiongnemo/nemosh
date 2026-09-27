// Package shellquote writes a string the ways bash writes one back out, so that a shell
// reading it gets the string it was: `printf %q`, `${x@Q}`, the values and keys of
// `declare -p`. bash is the reference for every one of them, measured; busybox has none.
//
// All of them share one rule. A string with a character that no other quoting can hold --
// a control character, a character that cannot be printed, a byte that is not UTF-8 -- is
// written whole as $'...'. Otherwise each form quotes in its own way.
package shellquote

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Backslash is `printf %q`: each character the shell treats specially behind a backslash,
// `a\ b`, and the empty string as a pair of single quotes.
func Backslash(value string) string {
	switch {
	case value == "":
		return "''"
	case needsANSIC(value):
		return ANSIC(value)
	}
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		if backslashed(value, index) {
			out.WriteByte('\\')
		}
		out.WriteByte(value[index])
	}
	return out.String()
}

// backslashed reports whether `printf %q` escapes the byte at index: a blank, a quote, an
// operator, a glob, an expansion or a brace, anywhere; a # where it would begin a comment;
// a ~ where tilde expansion would see it, first or after = or :.
func backslashed(value string, index int) bool {
	switch char := value[index]; {
	case strings.IndexByte(" \t\n!\"$&'()*,;<>?[\\]^`{|}", char) >= 0:
		return true
	case char == '#':
		return index == 0
	case char == '~':
		return tildeExpands(value, index)
	}
	return false
}

// Single is `${x@Q}`: the string in single quotes, with each quote in it written as a quote
// that ends them, a backslashed quote, and a quote that opens them again. One quote and
// nothing else is \'.
func Single(value string) string {
	switch {
	case value == "":
		return "''"
	case needsANSIC(value):
		return ANSIC(value)
	case value == "'":
		return `\'`
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// Double is a value as `declare -p` writes it: in double quotes, with a backslash before
// each ", \, $ and `.
func Double(value string) string {
	if needsANSIC(value) {
		return ANSIC(value)
	}
	var out strings.Builder
	out.WriteByte('"')
	for index := 0; index < len(value); index++ {
		if strings.IndexByte("\"\\$`", value[index]) >= 0 {
			out.WriteByte('\\')
		}
		out.WriteByte(value[index])
	}
	out.WriteByte('"')
	return out.String()
}

// Reusable is a value as bash's own `set` and a bare `declare` list it: as it is when the
// shell reads it back as itself, single-quoted when it holds a character the shell treats
// specially, and the empty value as nothing at all.
func Reusable(value string) string {
	switch {
	case needsANSIC(value):
		return ANSIC(value)
	case hasShellMeta(value):
		return Single(value)
	}
	return value
}

// Key is an associative array's key as `declare -p` writes it: bare when the shell reads it
// back as itself, and double-quoted when it holds a character the shell treats specially or
// is @ or * alone, which as a subscript would be every element.
func Key(key string) string {
	switch {
	case needsANSIC(key):
		return ANSIC(key)
	case key == "@" || key == "*" || hasShellMeta(key):
		return Double(key)
	}
	return key
}

// hasShellMeta reports a character the shell treats specially in a word: a blank, a quote,
// an operator, a glob, an expansion or a brace, or a # or ~ where each is special. A comma is
// not one, though `printf %q` escapes it.
func hasShellMeta(value string) bool {
	for index := 0; index < len(value); index++ {
		switch char := value[index]; {
		case strings.IndexByte(" \t\n'\"\\|&;()<>!{}*[?]^$`", char) >= 0:
			return true
		case char == '#' && index == 0, char == '~' && tildeExpands(value, index):
			return true
		}
	}
	return false
}

// tildeExpands reports a ~ at index where tilde expansion would see it: first in the word,
// or after an = or a :.
func tildeExpands(value string, index int) bool {
	return index == 0 || value[index-1] == '=' || value[index-1] == ':'
}

// ANSIC writes the string as $'...'. The control characters with an escape are written with
// it -- \n, \t, \E and the rest -- and a backslash or a quote behind a backslash. Every
// other byte that cannot be printed is \ooo. The rest, multibyte characters included, is
// written as itself.
func ANSIC(value string) string {
	var out strings.Builder
	out.WriteString("$'")
	for index := 0; index < len(value); {
		char, width := utf8.DecodeRuneInString(value[index:])
		switch {
		case char < utf8.RuneSelf:
			writeANSICByte(&out, value[index])
		case unprintable(char, width):
			for _, octet := range []byte(value[index : index+width]) {
				fmt.Fprintf(&out, "\\%03o", octet)
			}
		default:
			out.WriteString(value[index : index+width])
		}
		index += width
	}
	out.WriteByte('\'')
	return out.String()
}

var ansicEscapes = map[byte]string{
	0x07: `\a`, 0x08: `\b`, 0x09: `\t`, 0x0a: `\n`, 0x0b: `\v`, 0x0c: `\f`, 0x0d: `\r`,
	0x1b: `\E`, '\\': `\\`, '\'': `\'`,
}

func writeANSICByte(out *strings.Builder, char byte) {
	switch escape, ok := ansicEscapes[char]; {
	case ok:
		out.WriteString(escape)
	case char < 0x20 || char == 0x7f:
		fmt.Fprintf(out, "\\%03o", char)
	default:
		out.WriteByte(char)
	}
}

// needsANSIC reports a string only $'...' can write: one with a control character, a
// character that cannot be printed, or a byte that is not UTF-8.
func needsANSIC(value string) bool {
	for index := 0; index < len(value); {
		char, width := utf8.DecodeRuneInString(value[index:])
		if char < 0x20 || char == 0x7f || char >= utf8.RuneSelf && unprintable(char, width) {
			return true
		}
		index += width
	}
	return false
}

// unprintable is a character above ASCII that is not UTF-8, or that is not a letter, mark,
// number, punctuation, symbol or space: a C1 control, a format character like U+200B.
func unprintable(char rune, width int) bool {
	return char == utf8.RuneError && width == 1 || !unicode.IsGraphic(char)
}
