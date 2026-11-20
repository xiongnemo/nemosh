package applets

// Insert mode, busybox's char_insert. Escape goes back to command mode with the cursor one
// place back, as vi leaves it; so does ^C, as in vim. Backspace goes back past where the
// insert began and joins the line to the one before, as busybox's does; in R it puts back what
// was replaced. ^V puts the next key in as it is, and the Insert key turns an insert into R and
// back. The arrows and the keys like them move, and the insert goes on where they leave it, as
// in busybox. vim's where busybox puts them in as characters: ^W takes the word before the
// cursor, and ^U what this insert typed before it, then the rest of the line before it.

func (e *viEditor) insertKey(key rune) {
	if e.literalNext {
		e.literalNext = false
		if key >= 0 {
			e.insertRune(key)
		}
		return
	}
	switch {
	case key == viEscape || key == 3:
		e.dropAutoIndent()
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
	case key == viKeyInsert:
		mode := viReplace
		if e.mode == viReplace {
			mode = viInsert
		}
		e.mode, e.replaceStart, e.replaced = mode, e.col, nil
	case key < 0:
		e.normalKey(key)
		e.col = min(e.col, len(e.line()))
		e.insertStart, e.replaceStart, e.replaced = e.cursor(), e.col, nil
	case key == 22:
		e.literalNext = true
	case key == 4:
		e.outdent()
	case key == '\t':
		e.insertTab()
	case key == 23 || key == 21:
		e.eraseBefore(key)
	case key == 0:
	default:
		e.insertRune(key)
		if e.showMatch && (key == ')' || key == ']' || key == '}') {
			e.showMatching()
		}
	}
}

// insertRune puts r before the cursor, which moves past it; in replace mode, R, it takes the
// place of the character under the cursor, which Backspace puts back, and goes on after the
// line's end.
func (e *viEditor) insertRune(r rune) {
	line := e.line()
	grown := make([]rune, 0, len(line)+1)
	rest := line[e.col:]
	if e.mode == viReplace {
		original := rune(-1)
		if len(rest) > 0 {
			original, rest = rest[0], rest[1:]
		}
		e.replaced = append(e.replaced, original)
	}
	grown = append(append(append(grown, line[:e.col]...), r), rest...)
	e.replaceLines(e.row, e.row+1, [][]rune{grown})
	e.col++
}

// splitLine is Enter: the rest of the line goes onto a new one, and the cursor with it, after the
// indent autoindent gives it; a line that held only that indent leaves it to the new one.
func (e *viEditor) splitLine() {
	line := e.line()
	from := line
	if e.onlyAutoIndent() {
		e.replaceLines(e.row, e.row+1, [][]rune{{}})
		line, e.col = nil, 0
	}
	head := append([]rune(nil), line[:e.col]...)
	tail := append([]rune(nil), line[e.col:]...)
	e.replaceLines(e.row, e.row+1, [][]rune{head, tail})
	e.row, e.col = e.row+1, 0
	e.giveIndent(e.row, from)
}

// backspace takes the character before the cursor, or at the start of a line joins it to the
// one before. In replace mode it goes back only as far as the replace began, putting back what
// each character took the place of.
func (e *viEditor) backspace() {
	switch {
	case e.mode == viReplace && e.col > e.replaceStart && len(e.replaced) > 0:
		e.col--
		original := e.replaced[len(e.replaced)-1]
		e.replaced = e.replaced[:len(e.replaced)-1]
		line := append([]rune(nil), e.line()...)
		if original < 0 {
			line = append(line[:e.col], line[e.col+1:]...)
		} else {
			line[e.col] = original
		}
		e.replaceLines(e.row, e.row+1, [][]rune{line})
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

// eraseBefore is ^W and ^U, vim's: ^W back over blanks and then a word, or a run of punctuation;
// ^U back to where this insert began; each stopping first where the insert began if that comes
// before, and at the line's start joining it to the line before.
func (e *viEditor) eraseBefore(key rune) {
	if e.col == 0 {
		e.backspace()
		return
	}
	line, from := e.line(), e.col
	if key == 23 {
		for from > 0 && isViBlank(line[from-1]) {
			from--
		}
		if from > 0 {
			word := isViWord(line[from-1])
			for from > 0 && !isViBlank(line[from-1]) && isViWord(line[from-1]) == word {
				from--
			}
		}
	} else {
		from = 0
	}
	if start := e.insertStart; start.row == e.row && start.col < e.col && start.col > from {
		from = start.col
	}
	e.replaceLines(e.row, e.row+1, [][]rune{append(append([]rune(nil), line[:from]...), line[e.col:]...)})
	e.col = from
	if e.insertStart.row == e.row && e.insertStart.col > from {
		e.insertStart.col = from
	}
}
