package main

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

// completionStart is where the word Tab is completing begins: after the last
// blank before the cursor, with no walking back over blanks.
//
// Deliberately not wordStart. The two answer different questions and the blanks
// are where they part company: Ctrl-W deleting `echo   ` should remove `echo`,
// so wordStart steps over the trailing blanks to find it, while Tab after `cd `
// is completing a new and empty word. Sharing one boundary made the word under
// the cursor `"cd "`, which was completed as a command name, and nothing is
// called `"cd "` -- so the commonest gesture there is, a blank and then Tab, did
// nothing whatever.
// Scanned forwards rather than backwards, because an escape is only readable in
// that direction: walking back from the cursor, the blank in `My\ Documents`
// looks exactly like the one in `cd My`, and the word would be cut in half at a
// blank the user had already said was part of the name. busybox has the same
// problem and solves it the same way round, marking every `\c` before it looks
// for boundaries (libbb/lineedit.c:1155).
func (b *lineBuffer) completionStart() int {
	start := 0
	escaped := false
	for index := 0; index < b.cursor; index++ {
		if escaped {
			escaped = false
			continue
		}
		switch b.runes[index] {
		case '\\':
			escaped = true
		case ' ':
			start = index + 1
		}
	}
	return start
}

// currentWord is the text between the last blank and the cursor, which is what
// Tab completes. Empty when the cursor sits just after a blank, which is the
// case that says "offer me everything that could go here".
func (b *lineBuffer) currentWord() string {
	return string(b.runes[b.completionStart():b.cursor])
}

// currentWordPrefix is the text before the word being completed, which is what
// decides whether that word is a command name or an argument.
func (b *lineBuffer) currentWordPrefix() string {
	return string(b.runes[:b.completionStart()])
}

// wordEnd is the cursor position one word forward, for Alt-D and Alt-F.
func (b *lineBuffer) wordEnd() int {
	index := b.cursor
	for index < len(b.runes) && b.runes[index] == ' ' {
		index++
	}
	for index < len(b.runes) && b.runes[index] != ' ' {
		index++
	}
	return index
}

func (b *lineBuffer) moveWordLeft() { b.cursor = b.wordStart() }

func (b *lineBuffer) moveWordRight() { b.cursor = b.wordEnd() }

// deleteWordForward removes the word ahead of the cursor, which is readline's
// kill-word and busybox's Alt-D (libbb/lineedit.c:2926).
func (b *lineBuffer) deleteWordForward() {
	end := b.wordEnd()
	if end == b.cursor {
		return
	}
	b.runes = append(b.runes[:b.cursor], b.runes[end:]...)
}
