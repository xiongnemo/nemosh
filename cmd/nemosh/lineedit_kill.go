package main

// The kill ring: `^K`, `^U`, `^W` put text somewhere, and `^Y` takes it back.
//
// `^U` and `^W` already existed and simply **destroyed** what they removed, which is the
// half that makes them frightening to use. The pair is the point: `^U ... ^Y` moves a line
// you half-typed out of the way and back, and `^K ^Y` is how a line gets rearranged
// without retyping it. Nobody asks for a kill ring by name; they notice that `^U` lost
// something and stop pressing it.
//
// One slot rather than readline's ring of many, because everything beyond the first entry
// needs `M-y` to reach and a rotation state to explain, and the first entry is what the
// gesture is actually for. Named a ring anyway, because that is what it will grow into if
// it ever needs to.

// killRing holds the last killed text.
type killRing struct{ text string }

// kill stores text, unless there was none -- so a `^K` at the end of a line does not
// silently empty the ring and make the next `^Y` paste nothing.
func (k *killRing) kill(text string) {
	if text == "" {
		return
	}
	k.text = text
}

// yank answers what to paste.
func (k *killRing) yank() string { return k.text }

// killToEnd removes from the cursor to the end of the line and answers what it took.
//
// `^K`. The cursor does not move, because it is already where the text was.
func (b *lineBuffer) killToEnd() string {
	if b.cursor >= len(b.runes) {
		return ""
	}
	taken := string(b.runes[b.cursor:])
	b.runes = b.runes[:b.cursor]
	return taken
}

// killToStart removes from the start of the line to the cursor and answers what it took.
//
// `^U`. This is readline's unix-line-discard, which kills *backwards* rather than clearing
// the whole line -- so `^U` with the cursor in the middle keeps the tail. It used to clear
// everything, which is a different and more destructive gesture wearing the same key.
func (b *lineBuffer) killToStart() string {
	if b.cursor == 0 {
		return ""
	}
	taken := string(b.runes[:b.cursor])
	b.runes = append([]rune{}, b.runes[b.cursor:]...)
	b.cursor = 0
	return taken
}

// killWord removes the word before the cursor and answers what it took.
//
// `^W`, which deleteWord already did without saying what it removed.
func (b *lineBuffer) killWord() string {
	start := b.wordStart()
	if start == b.cursor {
		return ""
	}
	taken := string(b.runes[start:b.cursor])
	b.runes = append(b.runes[:start], b.runes[b.cursor:]...)
	b.cursor = start
	return taken
}

// yank inserts text at the cursor, leaving the cursor after it.
func (b *lineBuffer) yank(text string) {
	if text == "" {
		return
	}
	inserted := []rune(text)
	tail := append([]rune{}, b.runes[b.cursor:]...)
	b.runes = append(append(b.runes[:b.cursor], inserted...), tail...)
	b.cursor += len(inserted)
}
