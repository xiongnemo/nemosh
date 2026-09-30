package runtime

import (
	"errors"
	"fmt"
	"strings"
)

// A heredoc's operator and its word may be continued onto the next line with a backslash, and so
// may the line that ends its body, as busybox and bash both read them: `cat <\` then `<EOF`,
// `cat <<\` then `EOF`, `cat << EO\` then `F`, and in the body of one whose word is unquoted,
// `EO\` then `F` is the line EOF. The heredoc scan read each line on its own: it refused the
// first three as malformed redirections and never ended the fourth.

// errHeredocContinued is a line that ends inside a heredoc's operator or its word, with a
// backslash that the next line continues.
var errHeredocContinued = errors.New("heredoc operator continued on the next line")

// continuedHeredocDeclarations finds the heredocs the line at index declares, with the lines
// after it joined on while its operator or word goes on into them, and leaves index at the last
// line it took. The line it answers is the one it read, which is the output's: one line, whose
// number is its first's.
func continuedHeredocDeclarations(lines []string, index *int, startOrder int, scan heredocScan) (string, []pendingHeredoc, heredocScan, error) {
	start, line := *index, lines[*index]
	for {
		declarations, next, err := heredocDeclarations(line, start+1, startOrder, scan)
		if !errors.Is(err, errHeredocContinued) {
			return line, declarations, next, err
		}
		if *index+1 >= len(lines) {
			return line, nil, scan, fmt.Errorf("syntax error: %w", errMissingRedirectTarget)
		}
		*index++
		line = line[:len(line)-1] + lines[*index]
	}
}

// heredocBody takes a heredoc's body from the lines after index up to the one that is its
// delimiter, and leaves index there; the second answer is whether there was one.
//
// A line continued onto the next goes on inside it, so `<<-` takes the tabs from where the line
// begins and not from where it goes on: `\ta\` then `\tb` is a, a tab, and b, in both references.
func heredocBody(lines []string, index *int, declaration pendingHeredoc) (string, bool) {
	var body strings.Builder
	for *index++; *index < len(lines); *index++ {
		first := *index
		last, text := heredocLine(lines, first, declaration.expand)
		if declaration.stripTabs {
			text = strings.TrimLeft(text, "\t")
		}
		if text == declaration.delimiter {
			*index = last
			return body.String(), true
		}
		for line := first; line <= last; line++ {
			bodyLine := lines[line]
			if declaration.stripTabs && line == first {
				bodyLine = strings.TrimLeft(bodyLine, "\t")
			}
			body.WriteString(bodyLine)
			if line+1 < len(lines) {
				body.WriteByte('\n')
			}
		}
		*index = last
	}
	return body.String(), false
}

// heredocLine is the line of a heredoc's body that begins at index: where it ends, and its text
// with the backslash-newlines that join it gone. Only a heredoc that expands has them; in one
// whose word is quoted, a backslash is a backslash.
func heredocLine(lines []string, index int, expand bool) (int, string) {
	text := lines[index]
	for expand && continuesLine(text) && index+1 < len(lines) {
		index++
		text = text[:len(text)-1] + lines[index]
	}
	return index, text
}

// continuesLine is whether a line ends in a backslash that escapes its newline: an odd number of
// them, the last one unescaped.
func continuesLine(line string) bool {
	return (len(line)-len(strings.TrimRight(line, `\`)))%2 == 1
}
