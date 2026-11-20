package applets

// The edits everything else is made of. Each changes lines through replaceLines, which keeps the
// marks on the lines they were on, as vim's are kept; busybox's are pointers into its array that
// an edit before them leaves pointing at whatever moved there.

// replaceLines puts with in place of lines [from, to), and records it for undo. A buffer never
// has no lines: emptied, it is one empty line, as busybox adds one back.
func (e *viEditor) replaceLines(from, to int, with [][]rune) {
	if len(e.lines)-(to-from)+len(with) == 0 {
		with = [][]rune{{}}
	}
	e.record(from, copyLines(e.lines[from:to]), copyLines(with))
	rest := append([][]rune(nil), e.lines[to:]...)
	e.lines = append(append(e.lines[:from:from], with...), rest...)
	delta := len(with) - (to - from)
	for index := range e.marks {
		e.marks[index] = e.adjustMark(e.marks[index], from, to, delta, len(with))
	}
	e.context = e.adjustMark(e.context, from, to, delta, len(with))
}

// adjustMark is where a mark goes when lines [from, to) become count lines: on with the lines
// after them, onto the last of the new lines if its own went, and gone if they all did.
func (e *viEditor) adjustMark(mark viMark, from, to, delta, count int) viMark {
	switch {
	case !mark.set || mark.pos.row < from:
	case mark.pos.row >= to:
		mark.pos.row += delta
	case count == 0:
		mark.set = false
	case mark.pos.row-from >= count:
		mark.pos.row = from + count - 1
	}
	return mark
}

// textBetween is the runes from p to q, both included, a newline for each line's end.
func (e *viEditor) textBetween(p, q viPos) []rune {
	var text []rune
	for {
		text = append(text, e.charAt(p))
		if p == q {
			return text
		}
		next, ok := e.step(p, 1)
		if !ok {
			return text
		}
		p = next
	}
}

// cutChars takes the runes from p to q, both included, and answers them. A q at a line's end
// takes its newline, and the next line joins this one; the buffer's last newline stays.
func (e *viEditor) cutChars(p, q viPos) []rune {
	text := e.textBetween(p, q)
	last, tail := q.row, []rune(nil)
	switch {
	case q.col < len(e.lines[q.row]):
		tail = e.lines[q.row][q.col+1:]
	case q.row+1 < len(e.lines):
		last, tail = q.row+1, e.lines[q.row+1]
	default:
		text = text[:len(text)-1]
	}
	line := append(append([]rune(nil), e.lines[p.row][:p.col]...), tail...)
	e.replaceLines(p.row, last+1, [][]rune{line})
	return text
}

// linesText is lines [first, last] as a register holds them, a newline ending each.
func (e *viEditor) linesText(first, last int) []rune {
	var text []rune
	for _, line := range e.lines[first : last+1] {
		text = append(append(text, line...), '\n')
	}
	return text
}

// insertChars puts text, newlines and all, before p, and answers where its last rune went.
func (e *viEditor) insertChars(p viPos, text []rune) viPos {
	line := e.lines[p.row]
	head, tail := line[:p.col], line[p.col:]
	pieces := [][]rune{append([]rune(nil), head...)}
	for _, r := range text {
		if r == '\n' {
			pieces = append(pieces, nil)
			continue
		}
		pieces[len(pieces)-1] = append(pieces[len(pieces)-1], r)
	}
	last := len(pieces) - 1
	end := viPos{p.row + last, len(pieces[last]) - 1}
	pieces[last] = append(pieces[last], tail...)
	e.replaceLines(p.row, p.row+1, pieces)
	return end
}

// splitText is a register's text as lines, without the newline that ends the last.
func splitText(text []rune) [][]rune {
	var lines [][]rune
	start := 0
	for index, r := range text {
		if r == '\n' {
			lines = append(lines, append([]rune(nil), text[start:index]...))
			start = index + 1
		}
	}
	if start < len(text) {
		lines = append(lines, append([]rune(nil), text[start:]...))
	}
	return lines
}
