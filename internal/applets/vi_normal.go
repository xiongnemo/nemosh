package applets

// Command mode, busybox's do_cmd. A count before a command is typed as digits, 0 among them once
// one has begun. A key it does not implement says so, `'K' is not implemented`, as busybox's
// not_implemented does, rather than doing nothing.

func (e *viEditor) normalKey(key rune) {
	if e.pending != 0 {
		first := e.pending
		e.pending = 0
		e.secondKey(first, key)
	} else if key >= '1' && key <= '9' || key == '0' && e.count > 0 {
		e.count = min(e.count*10+int(key-'0'), 1<<24)
		return
	} else if !e.motionKey(key) {
		e.commandKey(key)
	}
	if e.pending == 0 {
		e.afterCommand()
	}
}

// afterCommand is the end of busybox's do_cmd: the count is spent, the cursor kept off a
// line's end in command mode, and the column j and k keep to is where the cursor now is,
// unless the command was one of those or $.
func (e *viEditor) afterCommand() {
	e.count = 0
	e.clampColumn()
	if !e.keepWant {
		e.want = e.displayColumn(e.line(), e.col)
	}
	e.keepWant = false
}

func (e *viEditor) commandKey(key rune) {
	switch key {
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
		e.mode, e.col = viInsert, e.firstNonBlank(e.row)
	case ':':
		e.mode, e.command = viCommandLine, []rune{':'}
	case 'Z', 'g', 'z', 'f', 'F', 't', 'T':
		e.pending = key
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

// secondKey is the key after one that waits for another. ZZ writes if the buffer changed and
// quits, :x, and ZQ quits without writing, :q!; gg is line count, or the first; z puts the
// cursor's line somewhere on the screen; f, F, t and T look for the key. Escape abandons any of
// them, as in vim.
func (e *viEditor) secondKey(first, key rune) {
	switch {
	case key == viEscape:
	case first == 'Z' && key == 'Z':
		e.exCommand("x")
	case first == 'Z' && key == 'Q':
		e.exCommand("q!")
	case first == 'g' && key == 'g':
		e.goToLine(max(1, e.count))
	case first == 'g':
		e.say(true, "'g%s' is not implemented", viKeyName(key))
	case first == 'z':
		e.redrawAround(key)
	case first == 'f' || first == 'F' || first == 't' || first == 'T':
		e.lastFind, e.findTarget = first, key
		e.findChar(first)
	default:
		e.beep()
	}
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
