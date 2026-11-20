package applets

// Command mode, busybox's do_cmd. A count before a command is typed as digits, 0 among them once
// one has begun. A key it does not implement says so, `'K' is not implemented`, as busybox's
// not_implemented does, rather than doing nothing.

func (e *viEditor) normalKey(key rune) {
	before := e.cursor()
	if e.pending == 0 && e.op == 0 && !e.repeating {
		// Each command is a change of its own to undo; see vi_undo.go.
		e.closeChange()
	}
	switch {
	case e.pending != 0:
		first := e.pending
		e.pending = 0
		e.secondKey(first, key)
	case key >= '1' && key <= '9' || key == '0' && e.count > 0:
		e.count = min(e.count*10+int(key-'0'), 1<<24)
		return
	case e.op != 0:
		e.operatorKey(key)
	case e.motionKey(key):
		if isJump(key) && e.cursor() != before {
			e.context = viMark{pos: before, set: true}
		}
	default:
		e.commandKey(key)
	}
	if e.prefix {
		// A register named is for the command still to come, with its count.
		e.prefix = false
		return
	}
	if e.pending == 0 && e.op == 0 {
		e.afterCommand()
	}
}

// afterCommand is the end of busybox's do_cmd: the count and the register named are spent, the
// cursor kept off a line's end in command mode, and the column j and k keep to is where the
// cursor now is, unless the command was one of those or $.
func (e *viEditor) afterCommand() {
	e.count, e.register, e.appending = 0, viDefaultRegister, false
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
	case ':', '/', '?':
		e.mode, e.command = viCommandLine, []rune{key}
	case 'Z', 'g', 'z', 'f', 'F', 't', 'T', '"', 'm', '\'', '`', 'r':
		e.pending = key
	case 'd', 'c', 'y', '<', '>', 'x', 'X', 's', 'S', 'D', 'C', 'Y':
		e.startOperator(key)
	case 'p', 'P':
		e.put(key)
	case 'J':
		e.join()
	case 'u':
		for range e.times() {
			e.undoChange()
		}
	case 18:
		for range e.times() {
			e.redoChange()
		}
	case 'U':
		e.restoreLine()
	case 'R':
		e.mode = viReplace
	case '~':
		e.toggleCase()
	case 'o', 'O':
		e.openLine(key)
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
		e.op = 0
	case e.op != 0:
		e.operate(first, key)
	case first == '"':
		e.chooseRegister(key)
		e.prefix = true
	case first == 'm':
		e.mark(key)
	case first == '\'' || first == '`':
		e.goToMark(first, key)
	case first == 'r':
		e.replaceChars(key)
	case first == 'Z' && key == 'Z':
		e.exCommand("x")
	case first == 'Z' && key == 'Q':
		e.exCommand("q!")
	case first == 'g' && key == 'g':
		e.context = viMark{pos: e.cursor(), set: true}
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
