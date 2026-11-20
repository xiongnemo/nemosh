package applets

// The screen's place in the buffer, busybox's screenbegin: what follows the cursor, and the
// commands that move it or move by it.

// scrollToCursor is busybox's sync_cursor: a cursor off the screen by no more than half of it
// scrolls it just far enough, and one further away is put in the screen's middle. Sideways, a
// long line is scrolled to keep the cursor on it, back to the start for a tab that begins it.
func (e *viEditor) scrollToCursor() {
	rows, half := e.textRows(), e.textRows()/2
	bottom := e.top + rows - 1
	switch {
	case e.row < e.top && e.top-e.row+1 > half, e.row > bottom && e.row-bottom+1 > half:
		e.top = max(0, e.row-half)
	case e.row < e.top:
		e.top = e.row
	case e.row > bottom:
		e.top += e.row - bottom
	}
	column := e.displayColumn(e.line(), e.col)
	if column < e.left {
		e.left = column
	}
	if column >= e.left+e.width {
		e.left = column - e.width + 1
	}
	if e.col == 0 && len(e.line()) > 0 && e.line()[0] == '\t' {
		e.left = 0
	}
}

// screenMotion is the commands that move by the screen, and reports whether key was one.
func (e *viEditor) screenMotion(key rune) bool {
	rows := e.textRows()
	switch key {
	case 'H':
		e.row = min(e.top+min(e.times(), rows)-1, len(e.lines)-1)
		e.col = e.firstNonBlank(e.row)
	case 'L':
		e.row = max(0, min(e.top+rows-1, len(e.lines)-1)-(min(e.times(), rows)-1))
		e.col = e.firstNonBlank(e.row)
	case 'M':
		e.row = min(e.top+rows/2, len(e.lines)-1)
		e.col = e.firstNonBlank(e.row)
	case 6, viKeyPageDown:
		e.scroll(rows-1, 1)
	case 2, viKeyPageUp:
		e.scroll(rows-1, -1)
	case 4:
		e.scroll((rows-1)/2, 1)
	case 21:
		e.scroll((rows-1)/2, -1)
	case 5:
		e.scroll(1, 1)
	case 25:
		e.scroll(1, -1)
	default:
		return false
	}
	return true
}

// scroll is busybox's dot_scroll: the screen moves by lines, and the cursor with it only as
// far as it must to stay on the screen, then past any blanks it is on.
func (e *viEditor) scroll(lines, dir int) {
	e.top = max(0, min(e.top+lines*dir, len(e.lines)-1))
	if e.row < e.top {
		e.row, e.col = e.top, 0
	}
	if bottom := min(e.top+e.textRows()-1, len(e.lines)-1); e.row > bottom {
		e.row, e.col = bottom, 0
	}
	for e.col < len(e.line()) && isViSpace(e.line()[e.col]) {
		e.col++
	}
}

// redrawAround is z: z. puts the cursor's line in the middle of the screen, z- at its bottom,
// and any other key at its top, as busybox's does.
func (e *viEditor) redrawAround(key rune) {
	e.top = e.row
	switch key {
	case '.':
		e.scroll((e.textRows()-1)/2, -1)
	case '-':
		e.scroll(e.textRows()-1, -1)
	default:
		e.scroll(0, -1)
	}
}
