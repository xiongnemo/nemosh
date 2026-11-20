package applets

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// :s with the c flag, vim's, busybox reading no flag but g: the cursor goes to each match and
// asks `replace with X (y/n/a/q/l)?` -- y replaces it, n passes it, a replaces it and every one
// after, q and Escape stop, l replaces it and stops. Without g it is a match a line. All of it is
// one change, the keys going to the question and not to commands.

// viConfirm is a :s///c going on: where the search goes on from, and what it has done.
type viConfirm struct {
	re          *regexp.Regexp
	replacement string
	global, all bool
	row, last   int
	col         int
	match       []int
	count       int
	lines       map[int]bool
}

func (e *viEditor) startConfirm(re *regexp.Regexp, replacement string, global bool, first, last int) {
	e.confirming = &viConfirm{re: re, replacement: replacement, global: global, row: first - 1, last: last - 1,
		lines: map[int]bool{}}
	e.nextConfirm()
}

// nextConfirm finds the next match and asks about it, or replaces it when a was typed, or ends.
func (e *viEditor) nextConfirm() {
	for c := e.confirming; c.row <= c.last && c.row < len(e.lines); c.row, c.col = c.row+1, 0 {
		line := string(e.lines[c.row])
		for _, match := range c.re.FindAllStringSubmatchIndex(line, -1) {
			start := utf8.RuneCountInString(line[:match[0]])
			if start < c.col {
				continue
			}
			e.row, e.col, c.match = c.row, start, match
			if c.all {
				e.replaceConfirmed()
				return
			}
			e.say(false, "replace with %s (y/n/a/q/l)?", c.replacement)
			return
		}
	}
	e.finishConfirm()
}

// confirmKey is the answer to the question.
func (e *viEditor) confirmKey(key rune) {
	c := e.confirming
	switch key {
	case 'y', 'l':
		e.replaceConfirmed()
		if key == 'l' && e.confirming != nil {
			e.finishConfirm()
		}
	case 'a':
		c.all = true
		e.replaceConfirmed()
	case 'n':
		c.col = e.col + 1
		if !c.global {
			c.row, c.col = c.row+1, 0
		}
		e.nextConfirm()
	case 'q', viEscape, 3:
		e.finishConfirm()
	default:
		e.say(false, "replace with %s (y/n/a/q/l)?", c.replacement)
	}
}

// replaceConfirmed replaces the match asked about, and goes on to the next.
func (e *viEditor) replaceConfirmed() {
	c := e.confirming
	line := string(e.lines[c.row])
	var out strings.Builder
	out.WriteString(line[:c.match[0]])
	expandReplacement(&out, c.replacement, line, c.match)
	before := out.String()
	pieces := strings.Split(before+line[c.match[1]:], "\n")
	lines := make([][]rune, len(pieces))
	for index, piece := range pieces {
		lines[index] = []rune(piece)
	}
	e.replaceLines(c.row, c.row+1, lines)
	c.count++
	c.lines[c.row] = true
	added := len(pieces) - 1
	c.last += added
	switch {
	case !c.global:
		c.row, c.col = c.row+added+1, 0
	default:
		c.row += added
		parts := strings.Split(before, "\n")
		c.col = utf8.RuneCountInString(parts[len(parts)-1])
		if c.match[0] == c.match[1] {
			c.col++
		}
	}
	e.nextConfirm()
}

// finishConfirm ends the question, saying how many it replaced as :s says it.
func (e *viEditor) finishConfirm() {
	c := e.confirming
	e.confirming = nil
	switch {
	case c.count == 0:
		e.say(true, "No match")
	case c.count > 1:
		e.say(false, "%d substitutions on %d lines", c.count, len(c.lines))
	default:
		e.message = ""
	}
	e.col = min(e.col, max(0, len(e.line())-1))
}
