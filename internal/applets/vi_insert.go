package applets

// Insert mode, busybox's char_insert. Escape goes back to command mode with the cursor one
// place back, as vi leaves it; so does ^C, as in vim. Backspace goes back past where the
// insert began and joins the line to the one before, as busybox's does. The arrows and the
// keys like them move, and the insert goes on where they leave it, as in busybox.

func (e *viEditor) insertKey(key rune) {
	switch {
	case key == viEscape || key == 3:
		e.mode = viNormal
		e.closeChange()
		if e.col > 0 {
			e.col--
		}
		e.clampColumn()
		e.want = e.displayColumn(e.line(), e.col)
	case key == viEnter || key == '\n':
		e.splitLine()
	case isViBackspace(key):
		e.backspace()
	case key < 0:
		e.normalKey(key)
		e.col = min(e.col, len(e.line()))
	case key == 0:
	default:
		e.insertRune(key)
	}
}

// insertRune puts r before the cursor, which moves past it; in replace mode, R, it takes the
// place of the character under the cursor, and goes on after the line's end.
func (e *viEditor) insertRune(r rune) {
	line := e.line()
	grown := make([]rune, 0, len(line)+1)
	rest := line[e.col:]
	if e.mode == viReplace && len(rest) > 0 {
		rest = rest[1:]
	}
	grown = append(append(append(grown, line[:e.col]...), r), rest...)
	e.replaceLines(e.row, e.row+1, [][]rune{grown})
	e.col++
}

// splitLine is Enter: the rest of the line goes onto a new one, and the cursor with it.
func (e *viEditor) splitLine() {
	line := e.line()
	head := append([]rune(nil), line[:e.col]...)
	tail := append([]rune(nil), line[e.col:]...)
	e.replaceLines(e.row, e.row+1, [][]rune{head, tail})
	e.row, e.col = e.row+1, 0
}

// backspace takes the character before the cursor, or at the start of a line joins it to the
// one before. In replace mode it only moves back.
func (e *viEditor) backspace() {
	switch {
	case e.mode == viReplace && e.col > 0:
		e.col--
	case e.col > 0:
		line := e.line()
		e.replaceLines(e.row, e.row+1, [][]rune{append(line[:e.col-1:e.col-1], line[e.col:]...)})
		e.col--
	case e.row > 0 && e.mode != viReplace:
		previous := e.lines[e.row-1]
		col := len(previous)
		joined := append(previous[:len(previous):len(previous)], e.line()...)
		e.replaceLines(e.row-1, e.row+1, [][]rune{joined})
		e.row, e.col = e.row-1, col
	default:
		e.beep()
	}
}
