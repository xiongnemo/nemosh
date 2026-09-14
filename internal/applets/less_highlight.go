package applets

import (
	"regexp"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// Highlighting what a search found.
//
// `/` moved to the match and then drew it like every other line, so there was nothing to
// say *where* on the line it had matched. busybox puts ESC[7m -- reverse video -- around
// every part of a line that matches rather than only the first
// (miscutils/less.c:867-889), and that is what this does.
//
// Its own file because less_view.go is within a few lines of the 250 ceiling, and because
// the compiling is shared: what gets highlighted and what `/` jumps to have to be the same
// question, and they were two copies of it.

// compiledPattern is the search as a regular expression.
//
// One place, so the highlight and the jump cannot disagree about what -I means or about
// which pattern is current.
func (p *lessPager) compiledPattern() (*regexp.Regexp, bool) {
	if p.pattern == "" {
		return nil, false
	}
	expression := p.pattern
	if p.settings.ignoreCase {
		expression = "(?i)" + expression
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return nil, false
	}
	return compiled, true
}

// matchSpans is where the pattern matches text, in **rune** positions shifted by offset.
//
// Runes rather than bytes because that is what a screen cell holds; offset is how far the
// line has been pushed right by something drawn before it, which is `-N`'s line number.
//
// An empty match is skipped, and busybox says why in its own words: "if even "" matches,
// treat it as not a match". A pattern like `x*` matches at every position, and highlighting
// those would paint the whole file.
func (p *lessPager) matchSpans(text string, offset int) [][2]int {
	compiled, ok := p.compiledPattern()
	if !ok {
		return nil
	}
	var spans [][2]int
	for _, match := range compiled.FindAllStringIndex(text, -1) {
		if match[0] == match[1] {
			continue
		}
		start := offset + utf8.RuneCountInString(text[:match[0]])
		end := start + utf8.RuneCountInString(text[match[0]:match[1]])
		spans = append(spans, [2]int{start, end})
	}
	return spans
}

// drawMatched draws one row, reversing the cells inside spans.
//
// Falls back to the plain path when there is nothing to highlight, which is every row of a
// pager nobody has searched in yet -- the common case, and one that should cost nothing.
func (p *lessPager) drawMatched(row int, text string, style tcell.Style, spans [][2]int) {
	if len(spans) == 0 {
		p.drawText(0, row, text, style)
		return
	}
	highlighted := style.Reverse(true)
	for column, character := range []rune(text) {
		if column >= p.width {
			return
		}
		p.screen.SetContent(column, row, character, nil, styleForColumn(style, highlighted, column, spans))
	}
}

// styleForColumn picks the style for one cell.
func styleForColumn(plain, highlighted tcell.Style, column int, spans [][2]int) tcell.Style {
	for _, span := range spans {
		if column >= span[0] && column < span[1] {
			return highlighted
		}
	}
	return plain
}
