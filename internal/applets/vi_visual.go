package applets

import (
	"math"
	"strings"
	"unicode"
)

// Visual mode, vim's, busybox having none: v selects characters and V lines, from where it was
// typed to the cursor, which any motion moves; o goes to the other end. d x y c s > < ~ J and r
// act on what is selected, X D Y C S and R on its lines, and : begins `:'<,'>`, the marks '< and
// '> left where it began and ended. Escape, or v again, leaves it. The selection is drawn in
// reverse, and the status line says `-- VISUAL --`.

// visualKey is a key while a selection is made, and reports whether it was one of visual mode's
// own; a motion is not, and moves the cursor as ever.
func (e *viEditor) visualKey(key rune) bool {
	switch key {
	case viEscape, 3:
		e.endVisual()
	case 'v', 'V':
		if e.visual == key {
			e.endVisual()
		} else {
			e.visual = key
		}
	case 'o':
		at := e.cursor()
		e.setCursor(e.visualStart)
		e.visualStart = at
	case 'r':
		e.pending = 'r'
	case ':':
		e.endVisual()
		e.mode, e.command = viCommandLine, []rune(":'<,'>")
	case 'd', 'x', 'X', 'D', 'y', 'Y', 'c', 's', 'C', 'S', 'R', '>', '<', '~', 'J':
		e.visualOperate(key)
	default:
		return false
	}
	return true
}

// selection is what is selected: its first and last place, and whether it is lines. After $ it
// takes the newline at the end of its last line, as vim's does.
func (e *viEditor) selection() (viPos, viPos, bool) {
	p, q := orderPositions(e.visualStart, e.cursor())
	if e.want == math.MaxInt && q == e.cursor() {
		q.col = len(e.lines[q.row])
	}
	return p, q, e.visual == 'V'
}

// visualOperate does key to the selection, and leaves visual mode.
func (e *viEditor) visualOperate(key rune) {
	p, q, linewise := e.selection()
	e.endVisual()
	if strings.ContainsRune("XDYCSR", key) {
		linewise = true
	}
	switch key {
	case '~':
		e.mapSelection(p, q, linewise, func(r rune) rune {
			if unicode.IsUpper(r) {
				return unicode.ToLower(r)
			}
			return unicode.ToUpper(r)
		})
		e.setCursor(p)
	case 'J':
		e.row, e.count = p.row, max(2, q.row-p.row+1)
		e.join()
	default:
		switch e.op = key; key {
		case 'x', 'X', 'D':
			e.op = 'd'
		case 'Y':
			e.op = 'y'
		case 's', 'C', 'S', 'R':
			e.op = 'c'
		}
		e.applyOperator(p, q, linewise)
	}
}

// replaceSelection is r in visual mode: every character selected becomes key.
func (e *viEditor) replaceSelection(key rune) {
	p, q, linewise := e.selection()
	e.endVisual()
	if key == viEscape || key == 3 {
		return
	}
	e.mapSelection(p, q, linewise, func(rune) rune { return key })
	e.setCursor(p)
}

// mapSelection changes each character from p to q, or of their lines, by change.
func (e *viEditor) mapSelection(p, q viPos, linewise bool, change func(rune) rune) {
	for row := p.row; row <= q.row; row++ {
		line := append([]rune(nil), e.lines[row]...)
		from, to := 0, len(line)
		if !linewise && row == p.row {
			from = p.col
		}
		if !linewise && row == q.row {
			to = min(q.col+1, len(line))
		}
		for index := from; index < to; index++ {
			line[index] = change(line[index])
		}
		e.replaceLines(row, row+1, [][]rune{line})
	}
}

// endVisual leaves visual mode, marking where the selection began and ended for '< and '>.
func (e *viEditor) endVisual() {
	if e.visual == 0 {
		return
	}
	p, q := orderPositions(e.visualStart, e.cursor())
	e.visualMarks = [2]viMark{{pos: p, set: true}, {pos: q, set: true}}
	e.visual = 0
}

// selected says whether the character at col of line row is in the selection being made.
func (e *viEditor) selected(row, col int) bool {
	if e.visual == 0 {
		return false
	}
	p, q, linewise := e.selection()
	switch {
	case row < p.row || row > q.row:
		return false
	case linewise:
		return true
	case row == p.row && col < p.col, row == q.row && col > q.col:
		return false
	}
	return true
}
