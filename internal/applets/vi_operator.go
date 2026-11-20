package applets

import "strings"

// The operators d c y < >, and what they reach: the same key again for lines, busybox's dd cc
// yy << >>, or a motion, ported from busybox's find_range. A count before the operator and one
// before the motion multiply, as busybox's get_motion_char has them.

// The motions as an operator reads them: by lines; up to the motion's end, not including it;
// including it. w, W, l and { } have rules of their own, below.
const (
	viLinewiseMotions  = "GHLM+-jk'g\r\n\x0e\x10"
	viExclusiveMotions = "^0bBFTh|`nN/?*#\x08\x7f"
	viInclusiveMotions = "$eEft%"
)

// combinedCount is the count before the operator times the count before the motion.
func (e *viEditor) combinedCount() int {
	switch {
	case e.opCount == 0:
		return e.count
	case e.count == 0:
		return e.opCount
	}
	return e.opCount * e.count
}

// operatorKey is the key after an operator.
func (e *viEditor) operatorKey(key rune) {
	switch {
	case key == viEscape || key == 3:
		e.op = 0
	case key == e.op:
		e.operateOnLines()
	case strings.ContainsRune("fFtT'`g", key):
		e.pending = key
	case key == '/' || key == '?':
		// The pattern is typed first; commandLineKey finishes the operator with it.
		e.mode, e.command = viCommandLine, []rune{key}
	default:
		e.operate(key, 0)
	}
}

// operateOnLines is dd, cc, yy, << and >>: count lines from the cursor's. A count that runs past
// the last line takes what there is, vim's, and none is taken from the last line itself, where
// busybox's j and vim refuse.
func (e *viEditor) operateOnLines() {
	count := max(1, e.combinedCount())
	last := e.row + count - 1
	if last >= len(e.lines) {
		if e.row == len(e.lines)-1 {
			e.beep()
			e.op = 0
			return
		}
		last = len(e.lines) - 1
	}
	e.applyOperator(e.cursor(), viPos{last, 0}, true)
}

// operate is an operator over a motion, arg the key a motion like f takes.
func (e *viEditor) operate(motion, arg rune) {
	start, count := e.cursor(), e.combinedCount()
	if e.op == 'c' && (motion == 'w' || motion == 'W') && !isViSpace(e.charAt(start)) {
		e.applyOperator(start, e.changeWordEnd(motion, max(1, count)), false)
		return
	}
	e.count, e.failed = count, false
	moved := e.runMotion(motion, arg)
	end := e.cursor()
	e.setCursor(start)
	// The column j and k keep to is where the operator leaves the cursor, as vim's is, not the
	// line's end a $ chose: busybox's d$ left it there, and the j after went to the next line's.
	e.count, e.keepWant = 0, false
	command := motion
	if motion == ';' || motion == ',' {
		command = e.lastFind
		if motion == ',' {
			command ^= 0x20
		}
	}
	switch {
	case !moved || e.failed:
	case motion == 'w' || motion == 'W':
		e.applyOperator(start, e.wordOperand(start, end), false)
		return
	case motion == 'l' || motion == ' ' || motion == viKeyRight:
		if end.row == start.row && end.col-start.col == max(1, count) {
			end.col--
		}
		e.applyOperator(start, end, false)
		return
	case strings.ContainsRune(viLinewiseMotions, command) || motion == viKeyUp || motion == viKeyDown:
		p, q := orderPositions(start, end)
		e.applyOperator(p, q, true)
		return
	case end == start:
	case motion == '{' || motion == '}':
		e.paragraphOperand(start, end)
		return
	case strings.ContainsRune(viExclusiveMotions, command) || motion == viKeyLeft || motion == viKeyHome:
		p, q := orderPositions(start, end)
		q, _ = e.step(q, -1)
		e.applyOperator(p, q, false)
		return
	case strings.ContainsRune(viInclusiveMotions, command) || motion == viKeyEnd:
		p, q := orderPositions(start, end)
		e.applyOperator(p, q, false)
		return
	}
	e.beep()
	e.op = 0
}

// runMotion runs a motion as an operator's, and reports whether it was one; the cursor is kept
// off a line's end, as busybox's do_cmd keeps it.
func (e *viEditor) runMotion(motion, arg rune) bool {
	switch motion {
	case 'f', 'F', 't', 'T':
		e.lastFind, e.findTarget = motion, arg
		e.findChar(motion)
	case '\'', '`':
		e.goToMark(motion, arg)
	case '/', '?':
		e.searchFor(motion, e.searchInput)
	case 'g':
		if arg != 'g' {
			return false
		}
		e.goToLine(max(1, e.count))
	default:
		if !e.motionKey(motion) {
			return false
		}
	}
	e.clampColumn()
	return true
}

// wordOperand is where dw and yw end, busybox's: back off the word w reached, unless w could go
// no further, and back over the blanks before it, but not over a newline unless all that is left
// is blank. busybox tested for the last word with at_eof, which a one-letter last word passes
// too, so dW before `c` took the c; vim's, and this, take only up to it.
func (e *viEditor) wordOperand(start, end viPos) viPos {
	q := end
	if isBefore(start, q) && !e.isLast(q) {
		q, _ = e.step(q, -1)
	}
	kept := q
	for isBefore(start, q) && isViSpace(e.charAt(q)) {
		was := e.charAt(q)
		q, _ = e.step(q, -1)
		if was == '\n' {
			kept = q
		}
	}
	if q != kept && e.charAt(q) != '\n' {
		q = kept
	}
	return q
}

// changeWordEnd is where cw and cW end on a word, vim's: at that word's end, as ce, unless the
// cursor is on its last character, which is then the first word changed.
func (e *viEditor) changeWordEnd(motion rune, count int) viPos {
	p := e.cursor()
	here, next := e.charAt(p), e.peek(p, 1)
	same := !isViSpace(next) && (motion == 'W' || isViWord(here) == isViWord(next) && isViPunct(here) == isViPunct(next))
	if !same {
		count--
	}
	for range count {
		if motion == 'W' {
			p = e.bigWordMotion('E', p)
		} else {
			p = e.wordMotion('e', p)
		}
	}
	return p
}

// paragraphOperand is busybox's { and } as an operator's: whole lines when the range begins a
// line and ends at an empty one or the end, and otherwise up to the empty line.
func (e *viEditor) paragraphOperand(start, end viPos) {
	p, q := orderPositions(start, end)
	last := len(e.lines) - 1
	atEnd := e.isLast(q) || q.row == last && q.col == len(e.lines[last])-1
	linewise := p.col == 0 && (e.charAt(q) == '\n' || atEnd)
	if !atEnd {
		q, _ = e.step(q, -1)
		if isBefore(p, q) && p.col != 0 {
			q, _ = e.step(q, -1)
		}
	}
	e.applyOperator(p, q, linewise)
}

func isBefore(a, b viPos) bool { return a.row < b.row || a.row == b.row && a.col < b.col }

func orderPositions(a, b viPos) (viPos, viPos) {
	if isBefore(b, a) {
		return b, a
	}
	return a, b
}
