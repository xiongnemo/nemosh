package applets

import "unicode"

// busybox's vi keeps its text in one array, a newline ending each line, and moves through it
// with a pointer; its motions cross lines because the pointer does. Here a line is a []rune, and
// a viPos is busybox's pointer: a rune of a line, or the line's end, where busybox has the
// newline. Its motions are ported onto that, so they cross lines where busybox's do.
type viPos struct{ row, col int }

func (e *viEditor) cursor() viPos { return viPos{e.row, e.col} }

func (e *viEditor) setCursor(p viPos) { e.row, e.col = p.row, p.col }

// charAt is the rune at p, or the newline at a line's end and outside the buffer.
func (e *viEditor) charAt(p viPos) rune {
	if p.row < 0 || p.row >= len(e.lines) || p.col < 0 || p.col >= len(e.lines[p.row]) {
		return '\n'
	}
	return e.lines[p.row][p.col]
}

// isFirst and isLast are busybox's text and end-1: the start of the buffer and the newline
// that ends it.
func (e *viEditor) isFirst(p viPos) bool { return p.row == 0 && p.col == 0 }

func (e *viEditor) isLast(p viPos) bool {
	return p.row == len(e.lines)-1 && p.col >= len(e.lines[p.row])
}

// step is p+dir, across a line's end; ok is false where that would leave the buffer.
func (e *viEditor) step(p viPos, dir int) (viPos, bool) {
	switch {
	case dir > 0 && p.col < len(e.lines[p.row]):
		return viPos{p.row, p.col + 1}, true
	case dir > 0 && p.row+1 < len(e.lines):
		return viPos{p.row + 1, 0}, true
	case dir < 0 && p.col > 0:
		return viPos{p.row, p.col - 1}, true
	case dir < 0 && p.row > 0:
		return viPos{p.row - 1, len(e.lines[p.row-1])}, true
	}
	return p, false
}

// peek is the rune after p in direction dir, or a newline past either end.
func (e *viEditor) peek(p viPos, dir int) rune {
	next, ok := e.step(p, dir)
	if !ok {
		return '\n'
	}
	return e.charAt(next)
}

// The classes busybox's C locale gives a byte, for runes: a word's, punctuation, and space,
// the newline among them. A letter of any script is a word's, as vim and the line editor have
// it; busybox, reading bytes, put a UTF-8 letter in none.
func isViSpace(r rune) bool { return unicode.IsSpace(r) }

func isViPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

func isViWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// viSkip is what busybox's skip_thing skips: up to a blank, over blanks, or to the end of a
// run of punctuation or of a word's characters.
type viSkip int

const (
	viBeforeSpace viSkip = iota
	viToSpace
	viOverSpace
	viEndPunct
	viEndWord
)

// skipThing is busybox's: it moves from p in direction dir while what it skips goes on, at
// most onto the lines'th newline, and never out of the buffer.
func (e *viEditor) skipThing(p viPos, lines, dir int, skip viSkip) viPos {
	for {
		c := e.charAt(p)
		var goes bool
		switch skip {
		case viBeforeSpace:
			c = e.peek(p, dir)
			goes = !isViSpace(c) || c == '\n'
		case viToSpace:
			goes = !isViSpace(c) || c == '\n'
		case viOverSpace:
			goes = isViSpace(c)
		case viEndPunct:
			c = e.peek(p, dir)
			goes = isViPunct(c)
		case viEndWord:
			c = e.peek(p, dir)
			goes = isViWord(c)
		}
		if !goes {
			return p
		}
		if c == '\n' {
			if lines--; lines < 1 {
				return p
			}
		}
		if dir > 0 && e.isLast(p) || dir < 0 && e.isFirst(p) {
			return p
		}
		p, _ = e.step(p, dir)
	}
}

// firstNonBlank is busybox's dot_begin and dot_skip_over_ws, the column of a line's first
// character that is not a blank.
func (e *viEditor) firstNonBlank(row int) int {
	line, col := e.lines[row], 0
	for col < len(line) && isViSpace(line[col]) {
		col++
	}
	return col
}
