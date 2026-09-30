package main

import "unicode"

// readline's editing commands that busybox's editor has not got, bound where bash binds
// them: C-t, M-. and M-_, M-u, M-l and M-c.

// transpose is transpose-chars, C-t: the character before the cursor dragged forward over the
// one at it, the cursor after both. At the end of the line the two before the cursor swap;
// at its start, or on a line of fewer than two, nothing happens.
func (b *lineBuffer) transpose() {
	if b.cursor == 0 || len(b.runes) < 2 {
		return
	}
	if b.cursor == len(b.runes) {
		b.cursor--
	}
	b.runes[b.cursor-1], b.runes[b.cursor] = b.runes[b.cursor], b.runes[b.cursor-1]
	b.cursor++
}

// changeWordCase is upcase-word, downcase-word and capitalize-word, M-u, M-l and M-c: the
// text from the cursor to the end of the next word, a word being letters and digits as
// readline's are, in upper case, lower case, or with each word's first letter upper and
// the rest lower. The cursor ends after the word.
func (b *lineBuffer) changeWordCase(operation rune) {
	start := b.cursor
	end := start
	for end < len(b.runes) && !isReadlineWordRune(b.runes[end]) {
		end++
	}
	for end < len(b.runes) && isReadlineWordRune(b.runes[end]) {
		end++
	}
	inWord := false
	for index := start; index < end; index++ {
		r := b.runes[index]
		if !isReadlineWordRune(r) {
			inWord = false
			continue
		}
		switch {
		case operation == 'u', operation == 'c' && !inWord:
			b.runes[index] = unicode.ToUpper(r)
		default:
			b.runes[index] = unicode.ToLower(r)
		}
		inWord = true
	}
	b.cursor = end
}

func isReadlineWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// lastArgument is what yank-last-arg keeps between presses: that the last key was one, how
// many lines further back than the one before the cursor it reached, and where what it
// inserted lies, so the next press can take it out again.
type lastArgument struct {
	active     bool
	skip       int
	start, end int
}

// yankLastArg is yank-last-arg, M-. and M-_: the last word of the line before the one being
// edited put in at the cursor, a word as `!$` has it. Pressed again straight after, it takes
// that out and puts in the last word of the line before that, and so on back. Past the oldest
// line it puts in nothing, as readline does.
func (e *lineEditor) yankLastArg() {
	if e.lastArg.active {
		e.buffer.runes = append(e.buffer.runes[:e.lastArg.start], e.buffer.runes[e.lastArg.end:]...)
		e.buffer.cursor = e.lastArg.start
		e.lastArg.skip++
	} else {
		e.lastArg.skip = 0
	}
	e.lastArg.active = true
	e.lastArg.start, e.lastArg.end = e.buffer.cursor, e.buffer.cursor
	index := len(e.history) - 1 - e.recall - e.lastArg.skip
	if index < 0 || index >= len(e.history) {
		return
	}
	if word, ok := historyArgs(lastWord, lastWord, e.history[index]); ok {
		e.buffer.yank(word)
		e.lastArg.end = e.buffer.cursor
	}
}
