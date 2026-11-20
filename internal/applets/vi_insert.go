package applets

// Insert mode, busybox's char_insert. Escape goes back to command mode with the cursor one
// place back, as vi leaves it; so does ^C, as in vim. Backspace goes back past where the
// insert began and joins the line to the one before, as busybox's does. The arrows and the
// keys like them move, and the insert goes on where they leave it, as in busybox.

func (e *viEditor) insertKey(key rune) {
	switch {
	case key == viEscape || key == 3:
		e.mode = viNormal
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

// insertRune puts r before the cursor, which moves past it.
func (e *viEditor) insertRune(r rune) {
	line := e.line()
	grown := make([]rune, 0, len(line)+1)
	grown = append(append(append(grown, line[:e.col]...), r), line[e.col:]...)
	e.lines[e.row] = grown
	e.col++
	e.changed()
}

// splitLine is Enter: the rest of the line goes onto a new one, and the cursor with it.
func (e *viEditor) splitLine() {
	line := e.line()
	head := append([]rune(nil), line[:e.col]...)
	tail := append([]rune(nil), line[e.col:]...)
	e.lines = append(e.lines[:e.row+1], append([][]rune{tail}, e.lines[e.row+1:]...)...)
	e.lines[e.row] = head
	e.row, e.col = e.row+1, 0
	e.changed()
}

// backspace takes the character before the cursor, or at the start of a line joins it to the
// one before.
func (e *viEditor) backspace() {
	switch {
	case e.col > 0:
		line := e.line()
		e.lines[e.row] = append(line[:e.col-1:e.col-1], line[e.col:]...)
		e.col--
	case e.row > 0:
		previous := e.lines[e.row-1]
		e.col = len(previous)
		e.lines[e.row-1] = append(previous[:len(previous):len(previous)], e.line()...)
		e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
		e.row--
	default:
		e.beep()
		return
	}
	e.changed()
}

// changed counts a change, which marks the buffer [Modified].
func (e *viEditor) changed() { e.modified++ }
