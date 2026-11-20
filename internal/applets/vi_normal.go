package applets

import "math"

// Command mode, busybox's do_cmd. A key it does not implement says so, `'K' is not
// implemented`, as busybox's not_implemented does, rather than doing nothing.

func (e *viEditor) normalKey(key rune) {
	if e.pending != 0 {
		first := e.pending
		e.pending = 0
		e.secondKey(first, key)
		return
	}
	switch key {
	case 'h', viKeyLeft, 8, viBackspace:
		e.moveColumn(-1)
	case 'l', ' ', viKeyRight:
		e.moveColumn(1)
	case 'j', viKeyDown, 10, 14:
		e.moveRow(1)
	case 'k', viKeyUp, 16:
		e.moveRow(-1)
	case '0', viKeyHome:
		e.col, e.want = 0, 0
	case '$', viKeyEnd:
		e.col, e.want = len(e.line()), math.MaxInt
		e.clampColumn()
	case 'i', viKeyInsert:
		e.mode = viInsert
	case 'a':
		e.mode = viInsert
		if len(e.line()) > 0 {
			e.col++
		}
	case 'A':
		e.mode, e.col = viInsert, len(e.line())
	case 'I':
		// Before the first character that is not a blank, busybox's and vim's.
		e.mode, e.col = viInsert, 0
		for e.col < len(e.line()) && (e.line()[e.col] == ' ' || e.line()[e.col] == '\t') {
			e.col++
		}
	case ':':
		e.mode, e.command = viCommandLine, []rune{':'}
	case 'Z':
		e.pending = 'Z'
	case 7:
		// ^G says where the cursor is, which the status line already does once a message has
		// gone.
		e.message = ""
	case 12:
		// ^L draws the screen again, from nothing.
		if e.screen != nil {
			e.screen.Sync()
		}
	case viEscape:
		e.beep()
	case 0, 3:
		// NUL is ignored, as busybox ignores it; ^C cancels what was being typed, as vim's does.
	default:
		e.say(true, "'%s' is not implemented", viKeyName(key))
	}
}

// secondKey is the key after one that waits for another: ZZ writes if the buffer changed and
// quits, :x; ZQ quits without writing, :q!; any other Z is an error.
func (e *viEditor) secondKey(first, key rune) {
	switch {
	case first == 'Z' && key == 'Z':
		e.exCommand("x")
	case first == 'Z' && key == 'Q':
		e.exCommand("q!")
	default:
		e.beep()
	}
}

// moveColumn moves along the line, stopping at either end.
func (e *viEditor) moveColumn(by int) {
	e.col += by
	e.clampColumn()
	e.want = e.displayColumn(e.line(), e.col)
}

// moveRow moves to another line, keeping the display column the cursor last chose.
func (e *viEditor) moveRow(by int) {
	row := e.row + by
	if row < 0 || row >= len(e.lines) {
		e.beep()
		return
	}
	e.row = row
	e.col = e.columnAt(e.want)
	e.clampColumn()
}

// columnAt is the rune of the cursor's line that covers display column want, or its last.
func (e *viEditor) columnAt(want int) int {
	line, column := e.line(), 0
	for index, r := range line {
		_, width := viGlyph(r, column, e.tabstop)
		if column+width > want {
			return index
		}
		column += width
	}
	return len(line)
}
