package runtime

import (
	goruntime "runtime"
	"strings"
)

// Windows programs end their lines with CRLF, and busybox-w32 reads those as lines: a command
// substitution's trailing newlines go with the carriage return of each CRLF among them, as in
// both references there -- `v=$(cmd /c ver)` ends where its text does -- and a field split from
// an unquoted expansion ends before a CRLF's CR, as busybox-w32 splits, so `set -- $(where git)`
// has no CR on its words. A CR anywhere else is a character, and every CR is one elsewhere than
// on Windows, as busybox and bash have it there. Each kept its CR: pip's completion offered
// `install` with one at its end.
var readsCRLFAsNewline = goruntime.GOOS == "windows"

// trimSubstitutionNewlines takes a command substitution's trailing newlines off, and on Windows
// the CR of each CRLF among them.
func trimSubstitutionNewlines(text string) string {
	for {
		switch {
		case readsCRLFAsNewline && strings.HasSuffix(text, "\r\n"):
			text = text[:len(text)-2]
		case strings.HasSuffix(text, "\n"):
			text = text[:len(text)-1]
		default:
			return text
		}
	}
}

// beforeNewline is a field's text without the CR of a CRLF it ends at, on Windows.
func beforeNewline(text string, next byte) string {
	if readsCRLFAsNewline && next == '\n' {
		return strings.TrimSuffix(text, "\r")
	}
	return text
}
