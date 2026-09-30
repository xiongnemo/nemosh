package applets

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// lsDisplayName is one entry's name as written: quoted or made printable as -Q and -q ask,
// painted when colour is on, with the -F or -p indicator after it.
//
// Both the short form and the long form go through here, because the indicator applies to
// every layout and two copies of "name plus indicator" is two answers eventually.
func lsDisplayName(entry lsEntry, options lsOptions) string {
	return paintLsName(lsNameText(entry.name, options), entry.info, options.colored) + classifyLsSuffix(entry.info, options)
}

// lsMeasuredName is the same text with no colour escapes, which is what the
// column grid has to measure: an escape is bytes that occupy no cells, and an
// indicator is one cell that must be counted. Getting this pair wrong shifts
// every column after the first coloured name.
func lsMeasuredName(entry lsEntry, options lsOptions) string {
	return lsNameText(entry.name, options) + classifyLsSuffix(entry.info, options)
}

// lsNameText is a name as busybox's print_name writes it. -Q puts it in double quotes, with a
// backslash before a quote or a backslash, \a to \r for the controls that have them, and three
// octal digits for any other byte that is no printable ASCII, a multibyte character's among
// them. -q, and a terminal, shows a `?` for each character that cannot be shown.
func lsNameText(name string, options lsOptions) string {
	switch {
	case options.quote:
		var quoted strings.Builder
		quoted.WriteByte('"')
		for index := 0; index < len(name); index++ {
			char := name[index]
			switch {
			case char == '"' || char == '\\':
				quoted.WriteByte('\\')
				quoted.WriteByte(char)
			case char >= 7 && char <= 13:
				quoted.WriteByte('\\')
				quoted.WriteByte("abtnvfr"[char-7])
			case char < ' ' || char > '~':
				quoted.WriteByte('\\')
				quoted.WriteString(string([]byte{'0' + char>>6, '0' + char>>3&7, '0' + char&7}))
			default:
				quoted.WriteByte(char)
			}
		}
		quoted.WriteByte('"')
		return quoted.String()
	case options.printable:
		return strings.Map(func(char rune) rune {
			if char == utf8.RuneError || !unicode.IsPrint(char) {
				return '?'
			}
			return char
		}, name)
	}
	return name
}

// classifyLsSuffix is the indicator: with -F `/` for a directory, `@` for a symlink, `*` for
// an executable, `|` for a pipe and `=` for a socket, and with -p the `/` alone, as busybox's
// append_char has them.
//
// It goes outside the colour escapes, which is where busybox puts it -- an
// indicator inside the reset would be coloured as though it were part of the
// name, and a script stripping the colour would keep it.
func classifyLsSuffix(info os.FileInfo, options lsOptions) string {
	if !options.classify && !options.slash || info == nil {
		return ""
	}
	switch {
	case info.IsDir():
		return "/"
	case !options.classify:
		return ""
	case info.Mode()&os.ModeSymlink != 0:
		return "@"
	case info.Mode()&os.ModeNamedPipe != 0:
		return "|"
	case info.Mode()&os.ModeSocket != 0:
		return "="
	case isExecutableEntry(info):
		// The same suffix list the shell uses for lookup, since Windows has no
		// execute bit to read. See isExecutableEntry in ls_color.go.
		return "*"
	}
	return ""
}
