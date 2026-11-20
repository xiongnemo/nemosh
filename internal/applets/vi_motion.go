package applets

import "math"

// The motions, busybox's do_cmd, each run `count` times. Two are vim's where a console robot
// found busybox's different (vi_motion_test.go): b from a line's start goes back over the
// newline to the word before, and a count that runs past the first or last line stops there
// rather than not moving at all.

// times is busybox's `do ... while (--cmdcnt > 0)`: the count, or once.
func (e *viEditor) times() int { return max(1, e.count) }

// motionKey moves the cursor if key is a motion, and reports whether it was one.
func (e *viEditor) motionKey(key rune) bool {
	switch key {
	case 'h', viKeyLeft, 8, viBackspace:
		e.col = max(0, e.col-e.times())
	case 'l', ' ', viKeyRight:
		e.col += e.times()
	case 'j', viKeyDown, 10, 14:
		e.moveLines(e.times(), false)
	case viEnter, '+':
		e.moveLines(e.times(), true)
	case 'k', viKeyUp, 16:
		e.moveLines(-e.times(), false)
	case '-':
		e.moveLines(-e.times(), true)
	case '0', viKeyHome:
		e.col = 0
	case '^':
		e.col = e.firstNonBlank(e.row)
	case '$', viKeyEnd:
		e.row = min(e.row+e.times()-1, len(e.lines)-1)
		e.col, e.want, e.keepWant = len(e.line()), math.MaxInt, true
	case '|':
		e.col = e.columnAt(e.count - 1)
	case 'w', 'b', 'e':
		for range e.times() {
			e.setCursor(e.wordMotion(key, e.cursor()))
		}
	case 'W', 'B', 'E':
		for range e.times() {
			e.setCursor(e.bigWordMotion(key, e.cursor()))
		}
	case 'G':
		e.goToLine(e.count)
	case '{', '}':
		e.paragraph(key)
	case '%':
		e.matchBracket()
	case ';':
		e.findChar(e.lastFind)
	case ',':
		e.findChar(e.lastFind ^ 0x20)
	case 'n':
		e.searchAgain(e.lastDirection)
	case 'N':
		e.searchAgain(-e.lastDirection)
	case '*', '#':
		e.searchWord(key)
	default:
		return e.screenMotion(key)
	}
	return true
}

// moveLines is j and k, and with firstNonBlank Enter, + and -: the column kept is the one the
// cursor last chose, or the end after $.
func (e *viEditor) moveLines(by int, firstNonBlank bool) {
	row := max(0, min(e.row+by, len(e.lines)-1))
	if row == e.row {
		e.beep()
		return
	}
	e.row = row
	switch {
	case firstNonBlank:
		e.col = e.firstNonBlank(row)
	case e.want == math.MaxInt:
		e.col, e.keepWant = len(e.line()), true
	default:
		e.col, e.keepWant = e.columnAt(e.want), true
	}
}

// goToLine is G: line count, or the last, at its first character that is not a blank.
func (e *viEditor) goToLine(count int) {
	e.row = len(e.lines) - 1
	if count > 0 {
		e.row = min(count, len(e.lines)) - 1
	}
	e.col = e.firstNonBlank(e.row)
}

// wordMotion is busybox's w, b and e. b goes over blanks as vim's does, a newline among them,
// stopping only at an empty line; busybox's stopped at the newline, on the line before's last
// character.
func (e *viEditor) wordMotion(key rune, p viPos) viPos {
	if key == 'w' {
		switch c := e.charAt(p); {
		case isViWord(c):
			p = e.skipThing(p, 1, 1, viEndWord)
		case isViPunct(c):
			p = e.skipThing(p, 1, 1, viEndPunct)
		}
		if !e.isLast(p) {
			p, _ = e.step(p, 1)
		}
		if isViSpace(e.charAt(p)) {
			p = e.skipThing(p, 2, 1, viOverSpace)
		}
		return p
	}
	dir := 1
	if key == 'b' {
		dir = -1
	}
	next, ok := e.step(p, dir)
	if !ok || dir > 0 && e.isLast(p) {
		return p
	}
	p = next
	if isViSpace(e.charAt(p)) {
		if key == 'b' {
			for isViSpace(e.charAt(p)) && len(e.lines[p.row]) > 0 && !e.isFirst(p) {
				p, _ = e.step(p, -1)
			}
		} else {
			p = e.skipThing(p, 2, dir, viOverSpace)
		}
	}
	switch c := e.charAt(p); {
	case isViWord(c):
		p = e.skipThing(p, 1, dir, viEndWord)
	case isViPunct(c):
		p = e.skipThing(p, 1, dir, viEndPunct)
	}
	return p
}

// bigWordMotion is busybox's W, B and E, whose words are only ever ended by blanks.
func (e *viEditor) bigWordMotion(key rune, p viPos) viPos {
	dir := 1
	if key == 'B' {
		dir = -1
	}
	if key == 'W' || isViSpace(e.peek(p, dir)) {
		p = e.skipThing(p, 1, dir, viToSpace)
		p = e.skipThing(p, 2, dir, viOverSpace)
	}
	if key != 'W' {
		p = e.skipThing(p, 1, dir, viBeforeSpace)
	}
	return p
}

// paragraph is busybox's { and }: to the next empty line in that direction, past any the
// cursor is in, or to the buffer's end.
func (e *viEditor) paragraph(key rune) {
	dir := 1
	if key == '{' {
		dir = -1
	}
	p := e.cursor()
	for range e.times() {
		skip, found := true, false
		for !found && (dir > 0 && !e.isLast(p) || dir < 0 && !e.isFirst(p)) {
			if e.charAt(p) == '\n' && e.peek(p, dir) == '\n' {
				if !skip {
					if dir > 0 {
						p, _ = e.step(p, 1)
					}
					found = true
					continue
				}
			} else {
				skip = false
			}
			p, _ = e.step(p, dir)
		}
	}
	e.setCursor(p)
}

// findChar is busybox's dot_to_char: f and t forward, F and T back, count times, within the
// line; t stops before the character and T after it. Not found, the cursor stays.
func (e *viEditor) findChar(command rune) {
	if e.findTarget == 0 {
		return
	}
	dir := 1
	if command == 'F' || command == 'T' {
		dir = -1
	}
	line, col := e.line(), e.col
	for range e.times() {
		for {
			col += dir
			if col < 0 || col >= len(line) {
				e.beep()
				return
			}
			if line[col] == e.findTarget {
				break
			}
		}
	}
	switch command {
	case 't':
		col--
	case 'T':
		col++
	}
	e.col = col
}
