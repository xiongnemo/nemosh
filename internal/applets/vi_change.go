package applets

import (
	"strings"
	"unicode"
)

// What an operator does to its range, and the commands that change text without one.

// applyOperator does the pending operator to p through q, by lines or not. d and y say what they
// took, busybox's yank_status; c and the one-key forms of d and c do not.
func (e *viEditor) applyOperator(p, q viPos, linewise bool) {
	op, operation := e.op, ""
	switch {
	case e.quiet:
	case op == 'd':
		operation = "Delete"
	case op == 'y':
		operation = "Yank"
	}
	e.op, e.quiet = 0, false
	switch {
	case op == '<' || op == '>':
		e.shiftLines(op, p.row, q.row)
		e.row, e.col = p.row, e.firstNonBlank(p.row)
	case linewise:
		e.store(operation, e.linesText(p.row, q.row), true)
		switch op {
		case 'y':
			e.setCursor(p)
		case 'd':
			e.replaceLines(p.row, q.row+1, nil)
			e.row = min(p.row, len(e.lines)-1)
			e.col = e.firstNonBlank(e.row)
		case 'c':
			// With autoindent the line keeps its indent, busybox's newindent.
			first := append([]rune(nil), e.lines[p.row]...)
			e.replaceLines(p.row, q.row+1, [][]rune{{}})
			e.row, e.col = p.row, 0
			e.enterInsert(viInsert)
			e.giveIndent(p.row, first)
		}
	case e.charAt(p) == '\n' && p == q:
		// busybox's yank_delete takes nothing from an empty line, or from a line's end; a
		// change there only inserts.
		if op == 'c' {
			e.setCursor(p)
			e.enterInsert(viInsert)
		}
	case op == 'y':
		e.store(operation, e.textBetween(p, q), false)
		e.setCursor(p)
	default:
		e.store(operation, e.cutChars(p, q), false)
		e.setCursor(p)
		if op == 'c' {
			e.enterInsert(viInsert)
		}
	}
}

// startOperator is d, c, y, < or > typed, or one of the keys that is one with its motion: x is
// dl, X dh, s cl, D d$, C c$, Y yy and S cc, vim's, busybox having no S.
func (e *viEditor) startOperator(key rune) {
	e.opCount, e.count = e.count, 0
	shortcut := ""
	if at := strings.IndexRune("xXsDCYS", key); at >= 0 {
		shortcut = [...]string{"dl", "dh", "cl", "d$", "c$", "yy", "cc"}[at]
	}
	if shortcut == "" {
		e.op = key
		return
	}
	e.op, e.quiet = rune(shortcut[0]), key != 'Y'
	motion := rune(shortcut[1])
	switch {
	case motion == e.op:
		e.operateOnLines()
	case key == 'D' || key == 'C':
		// To the end of the line only; the count is busybox's, ignored.
		e.opCount = 0
		end := viPos{e.row, max(0, len(e.line())-1)}
		if len(e.line()) == 0 {
			end.col = 0
		}
		e.applyOperator(e.cursor(), end, false)
	default:
		e.operate(motion, 0)
	}
}

// shiftLines is < and >, busybox's: > puts a tab before each line that is not empty, and <
// takes away a tab, or up to a tabstop of spaces.
func (e *viEditor) shiftLines(op rune, first, last int) {
	for row := first; row <= last && row < len(e.lines); row++ {
		line := e.lines[row]
		switch {
		case op == '>' && len(line) > 0:
			e.replaceLines(row, row+1, [][]rune{append([]rune{'\t'}, line...)})
		case op == '<' && len(line) > 0 && line[0] == '\t':
			e.replaceLines(row, row+1, [][]rune{append([]rune(nil), line[1:]...)})
		case op == '<':
			spaces := 0
			for spaces < len(line) && spaces < e.tabstop && line[spaces] == ' ' {
				spaces++
			}
			if spaces > 0 {
				e.replaceLines(row, row+1, [][]rune{append([]rune(nil), line[spaces:]...)})
			}
		}
	}
}

// put is p and P, busybox's: lines go below or above the cursor's, the cursor onto the first
// that is not blank; other text after or at the cursor, the cursor on its last character, as
// vim leaves it where busybox went on past blanks.
func (e *viEditor) put(key rune) {
	register := e.registers[e.register]
	if len(register.text) == 0 {
		e.say(true, "Nothing in register %c", registerName(e.register))
		return
	}
	times := e.times()
	text := []rune(strings.Repeat(string(register.text), times))
	if register.linewise {
		row := e.row
		if key == 'p' {
			row++
		}
		e.replaceLines(row, row, splitText(text))
		e.row, e.col = row, e.firstNonBlank(row)
	} else {
		at := e.cursor()
		if key == 'p' && len(e.line()) > 0 {
			at.col++
		}
		e.setCursor(e.insertChars(at, text))
		if strings.ContainsRune(string(register.text), '\n') {
			e.setCursor(at)
		}
	}
	e.sayTaken("Put", register.text, times, e.register)
}

// join is J, vim's: count lines, two at the least, each after the first with its leading blanks
// gone and joined by one space -- none where the line before ends in a blank, the next begins
// with ), or either is empty -- the cursor where the last join was made. busybox joined count+1
// lines and always put a space in.
func (e *viEditor) join() {
	count := max(2, e.times())
	if e.row == len(e.lines)-1 {
		e.beep()
		return
	}
	last := min(e.row+count-1, len(e.lines)-1)
	line, col := append([]rune(nil), e.line()...), 0
	for _, next := range e.lines[e.row+1 : last+1] {
		next = []rune(strings.TrimLeftFunc(string(next), isViBlank))
		col = len(line)
		ends := len(line) > 0 && isViBlank(line[len(line)-1])
		if len(line) > 0 && len(next) > 0 && !ends && next[0] != ')' {
			line = append(line, ' ')
		}
		line = append(line, next...)
	}
	e.replaceLines(e.row, last+1, [][]rune{line})
	e.col = col
}

func isViBlank(r rune) bool { return r == ' ' || r == '\t' }

// replaceChars is r: count characters become key, an error if the line has fewer; r<Enter>
// takes them and breaks the line there.
func (e *viEditor) replaceChars(key rune) {
	count, line := e.times(), e.line()
	if key == viEscape || key == 3 {
		return
	}
	if e.col+count > len(line) {
		e.beep()
		return
	}
	if key == viEnter || key == '\n' {
		head := append([]rune(nil), line[:e.col]...)
		tail := append([]rune(nil), line[e.col+count:]...)
		e.replaceLines(e.row, e.row+1, [][]rune{head, tail})
		e.row, e.col = e.row+1, 0
		return
	}
	changed := append([]rune(nil), line...)
	for index := e.col; index < e.col+count; index++ {
		changed[index] = key
	}
	e.replaceLines(e.row, e.row+1, [][]rune{changed})
	e.col += count - 1
}

// toggleCase is ~: count characters change case, and the cursor goes past them.
func (e *viEditor) toggleCase() {
	line := append([]rune(nil), e.line()...)
	end := min(e.col+e.times(), len(line))
	for index := e.col; index < end; index++ {
		if unicode.IsUpper(line[index]) {
			line[index] = unicode.ToLower(line[index])
		} else {
			line[index] = unicode.ToUpper(line[index])
		}
	}
	e.replaceLines(e.row, e.row+1, [][]rune{line})
	e.col = end
}

// openLine is o and O: a new line below or above, and an insert on it, with the cursor's line's
// indent when autoindent is set, as busybox gives it.
func (e *viEditor) openLine(key rune) {
	row, from := e.row, append([]rune(nil), e.line()...)
	if key == 'o' {
		row++
	}
	e.replaceLines(row, row, [][]rune{{}})
	e.row, e.col = row, 0
	e.enterInsert(viInsert)
	e.giveIndent(row, from)
}
