package main

import "strings"

// lineBuffer is one line being edited: the runes in it and where the cursor
// sits between them.
//
// Runes, not bytes, and not columns. Editing is by character -- one backspace
// removes one character whatever it costs on screen -- while drawing is by
// column. Conflating the two is the bug in busybox's editor, where backspacing
// over a CJK character moves the cursor one cell and leaves the other half.
type lineBuffer struct {
	runes  []rune
	cursor int
}

func newLineBuffer() *lineBuffer {
	return &lineBuffer{}
}

// insert adds a printable rune at the cursor. Control characters and tab are
// refused: the editor handles those as keys, and a literal tab in the buffer
// would make the drawn width depend on the terminal's tab stops.
func (b *lineBuffer) insert(r rune) {
	if r < 0x20 || r == 0x7f {
		return
	}
	b.runes = append(b.runes, 0)
	copy(b.runes[b.cursor+1:], b.runes[b.cursor:])
	b.runes[b.cursor] = r
	b.cursor++
}

// backspace removes the whole character before the cursor.
func (b *lineBuffer) backspace() {
	if b.cursor == 0 {
		return
	}
	b.runes = append(b.runes[:b.cursor-1], b.runes[b.cursor:]...)
	b.cursor--
}

// deleteForward removes the character at the cursor, which is what Delete does
// and what Ctrl-D means on a line that is not empty.
func (b *lineBuffer) deleteForward() {
	if b.cursor >= len(b.runes) {
		return
	}
	b.runes = append(b.runes[:b.cursor], b.runes[b.cursor+1:]...)
}

func (b *lineBuffer) moveLeft() {
	if b.cursor > 0 {
		b.cursor--
	}
}

func (b *lineBuffer) moveRight() {
	if b.cursor < len(b.runes) {
		b.cursor++
	}
}

func (b *lineBuffer) moveHome() { b.cursor = 0 }

func (b *lineBuffer) moveEnd() { b.cursor = len(b.runes) }

// replace swaps the whole line, which is what recalling a history entry does.
// The cursor goes to the end, where a recalled line is usually edited from.
func (b *lineBuffer) replace(text string) {
	b.runes = []rune(text)
	b.cursor = len(b.runes)
}

func (b *lineBuffer) String() string {
	return string(b.runes)
}

func (b *lineBuffer) isEmpty() bool { return len(b.runes) == 0 }

func (b *lineBuffer) runeCount() int { return len(b.runes) }

// columns is the width of the whole line on screen.
func (b *lineBuffer) columns() int {
	total := 0
	for _, r := range b.runes {
		total += runeColumns(r)
	}
	return total
}

// cursorColumns is the width of everything left of the cursor, which is where
// the terminal cursor has to be put after a redraw.
func (b *lineBuffer) cursorColumns() int {
	total := 0
	for _, r := range b.runes[:b.cursor] {
		total += runeColumns(r)
	}
	return total
}

// wordStart is the cursor position one word back, for Ctrl-W.
func (b *lineBuffer) wordStart() int {
	index := b.cursor
	for index > 0 && b.runes[index-1] == ' ' {
		index--
	}
	for index > 0 && b.runes[index-1] != ' ' {
		index--
	}
	return index
}

// deleteWord removes the word before the cursor.
func (b *lineBuffer) deleteWord() {
	start := b.wordStart()
	if start == b.cursor {
		return
	}
	b.runes = append(b.runes[:start], b.runes[b.cursor:]...)
	b.cursor = start
}

// currentWord is the text between the last blank and the cursor, which is what
// Tab completes.
func (b *lineBuffer) currentWord() string {
	return strings.TrimLeft(string(b.runes[b.wordStart():b.cursor]), " ")
}
