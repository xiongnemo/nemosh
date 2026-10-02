package main

// The kill ring: `^K`, `^U`, `^W` put text somewhere, and `^Y` takes it back.
//
// `^U` and `^W` already existed and simply **destroyed** what they removed, which is the
// half that makes them frightening to use. The pair is the point: `^U ... ^Y` moves a line
// you half-typed out of the way and back, and `^K ^Y` is how a line gets rearranged
// without retyping it. Nobody asks for a kill ring by name; they notice that `^U` lost
// something and stop pressing it.
//
// It is readline's ring: the last ten kills, a kill straight after another joined to it --
// two ^W are one entry, the words in order -- and M-y straight after a ^Y taking the yank
// back out and putting in the kill before it, round the ring. It held one kill, the newest,
// which is what ^Y is mostly for; what it lost was what M-y is for.

// maxKills is readline's DEFAULT_MAX_KILLS.
const maxKills = 10

// killRing holds what the kill keys took, newest last.
type killRing struct {
	entries []string
	// back is which entry ^Y puts in, counted back from the newest: M-y moves it on, and
	// it stays moved, as readline's rl_kill_index does, until the next kill.
	back int
	// joining is whether the key before was a kill, which a kill joins.
	joining bool
	// yanked is where the last ^Y or M-y put its text, for M-y to take it back out; active
	// only straight after one of them.
	yanked yankSpan
}

type yankSpan struct {
	active     bool
	start, end int
}

// kill stores text, joined to the newest entry when the key before was a kill too: after
// it when it came from after the cursor, before it when from before, so the entry reads
// as the line did. Nothing killed stores nothing -- a `^K` at the end of a line does not
// silently empty the ring and make the next `^Y` paste nothing.
func (k *killRing) kill(text string, backward bool) {
	if text == "" {
		return
	}
	k.back = 0
	newest := len(k.entries) - 1
	switch {
	case k.joining && newest >= 0 && backward:
		k.entries[newest] = text + k.entries[newest]
	case k.joining && newest >= 0:
		k.entries[newest] += text
	default:
		k.entries = append(k.entries, text)
		if len(k.entries) > maxKills {
			k.entries = k.entries[1:]
		}
	}
}

// yank answers what to paste.
func (k *killRing) yank() string {
	if len(k.entries) == 0 {
		return ""
	}
	return k.entries[len(k.entries)-1-k.back]
}

// rotate moves ^Y's entry one further back, round the ring, and answers it.
func (k *killRing) rotate() string {
	k.back = (k.back + 1) % len(k.entries)
	return k.yank()
}

// yank is ^Y: the kill ring's entry put in at the cursor, and where it went noted, for M-y.
func (e *lineEditor) yank() {
	start := e.buffer.cursor
	e.buffer.yank(e.kills.yank())
	e.kills.yanked = yankSpan{active: len(e.kills.entries) > 0, start: start, end: e.buffer.cursor}
}

// yankPop is M-y: straight after ^Y or M-y, the text it put in taken back out and the kill
// before it put in instead, round the ring; anywhere else nothing, as readline has it.
func (e *lineEditor) yankPop() {
	span := e.kills.yanked
	if !span.active {
		return
	}
	e.buffer.runes = append(e.buffer.runes[:span.start:span.start], e.buffer.runes[span.end:]...)
	e.buffer.cursor = span.start
	e.buffer.yank(e.kills.rotate())
	e.kills.yanked.end = e.buffer.cursor
}

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
