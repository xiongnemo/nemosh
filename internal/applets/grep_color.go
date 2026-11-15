package applets

import (
	"io"
	"regexp"
	"strings"
)

// --color, which colours: it was accepted and ignored, as busybox's is, so the
// `alias grep='grep --color=auto'` that every GNU system has, and nemoshrc's example has,
// coloured nothing. GNU's defaults, byte for byte as GNU grep 3.0 writes them, each colour
// followed by an erase to the end of the line and ended the same way:
//
//	file name 35   line number 32   separator 36   match 01;31
//
// `auto` is a terminal only, resolved against the stream written to, as ls resolves it. A match
// is coloured in a selected line; a context line, and a line -v selected, has none to colour.
// GREP_COLORS is not read.

const (
	grepColorName      = "35"
	grepColorLine      = "32"
	grepColorSeparator = "36"
	grepColorMatch     = "01;31"
)

// newGrepPrinter is the printer for one run, colouring when --color says to.
func newGrepPrinter(stdout, stderr io.Writer, flags grepFlags, expr *regexp.Regexp) *grepPrinter {
	return &grepPrinter{stdout: stdout, stderr: stderr, flags: flags, expr: expr, color: colorEnabled(flags.color, stdout)}
}

// paint is text in a colour, when this run colours.
func (p *grepPrinter) paint(code, text string) string {
	if !p.color || text == "" {
		return text
	}
	return "\x1b[" + code + "m\x1b[K" + text + "\x1b[m\x1b[K"
}

// highlight is a selected line with every match in it coloured. Under -w the match is the word
// alone, not the characters the pattern holds on either side of it; see grepWords.
func (p *grepPrinter) highlight(text string) string {
	if !p.color || p.flags.invert || p.expr == nil {
		return text
	}
	var out strings.Builder
	last := 0
	for _, span := range grepMatchSpans(p.expr, p.flags.wordMatch, text) {
		out.WriteString(text[last:span[0]])
		out.WriteString(p.paint(grepColorMatch, text[span[0]:span[1]]))
		last = span[1]
	}
	out.WriteString(text[last:])
	return out.String()
}

// grepMatchSpans is where the pattern matches text, by byte, the empty matches left out, as an
// empty match is not printed by -o either; under -w, the words, as -o prints them.
func grepMatchSpans(expr *regexp.Regexp, word bool, text string) [][2]int {
	if word {
		return grepWordSpans(expr, text)
	}
	var spans [][2]int
	for _, match := range expr.FindAllStringIndex(text, -1) {
		if match[0] < match[1] {
			spans = append(spans, [2]int{match[0], match[1]})
		}
	}
	return spans
}
