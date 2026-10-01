package main

import (
	"time"
	"unicode"
)

// vi editing mode, `set -o vi`, as busybox's editor has it (libbb/lineedit.c,
// FEATURE_EDITING_VI, which busybox-w32 builds in).
//
// A line starts in insert mode, where the keys are the ones the editor always had. Escape
// goes to command mode, and the cursor one place back, as vi leaves it. There the keys are
// busybox's:
//
//	i I a A          insert: here, at the start, after the cursor, at the end
//	x X              delete the character under the cursor, before it
//	w W e E b B      word motions, busybox's: w and e stop at punctuation, W and E at blanks
//	0 $ h l space    start, end, left, right; Backspace goes left too
//	j k              the next and the previous history line, which start at their beginning
//	D C              delete to the end, and C goes on to insert
//	d c + motion     delete over w W e E b B, a space, or to the end with $; dd and cc
//	                 the whole line; c goes on to insert
//	p P              put back what the last deletion took, after the cursor or before it
//	r                replace the character under the cursor with the next one typed
//
// Enter, ^C, ^D, ^L, ^U, ^W, ^N, ^P, Delete and the arrows, Home and End, Ctrl with Left and
// Right, and Page Up and Down do in command mode what they do in insert mode. Any other key
// does nothing there, so a letter never lands in the line by accident; a line starts in insert
// mode again, and with nothing to put.
//
// The keys arrive from a terminal that sends Escape at the start of its sequences, which
// busybox-w32, reading the console's key events, never sees. So an Escape is the start of one
// only with `[` or `O` straight after it, and one that nothing follows within 50 ms is the key
// on its own, as busybox's read_key waits on a terminal; Alt with a letter, which is Escape and
// the letter, is the two keys here.

// viState is the editor's vi mode: whether it is on, whether the line is in command mode,
// an operator waiting for the key it applies to, and what the last deletion took.
type viState struct {
	on, command bool
	pending     rune
	deleted     []rune
}

// escapeWait is how long a lone Escape waits for the rest of an escape sequence before it is a
// key of its own: busybox's read_key waits 50 ms.
const escapeWait = 50 * time.Millisecond

// viKey handles k when the editor is in vi mode, and reports whether it did; a key it leaves
// is the ordinary bindings'.
func (e *lineEditor) viKey(k key) bool {
	v := &e.vi
	if v.pending != 0 {
		e.viOperand(k)
		return true
	}
	if k.kind == keyEscape {
		if !v.command {
			v.command = true
			e.buffer.moveLeft()
		}
		return true
	}
	// A history line starts at its beginning in vi mode, in either mode, as busybox draws it.
	if k.kind == keyUp || k.kind == keyDown {
		direction, before := 1, e.recall
		if k.kind == keyDown {
			direction = -1
		}
		e.recallHistory(direction)
		if e.recall != before {
			e.buffer.moveHome()
		}
		return true
	}
	if !v.command {
		return false
	}
	switch k.kind {
	case keyEnter, keyInterrupt, keyEndOfInput, keyLeft, keyRight, keyHome, keyEnd, keyDelete,
		keyWordLeft, keyWordRight, keyClearLine, keyDeleteWord, keyClearScreen,
		keyHistoryPrefixBackward, keyHistoryPrefixForward:
		return false
	case keyBackspace:
		e.buffer.moveLeft()
	case keyRune:
		e.viCommand(k.value)
	}
	return true
}

// viCommand is a key typed in command mode.
func (e *lineEditor) viCommand(command rune) {
	v, b := &e.vi, e.buffer
	switch command {
	case 'i':
		v.command = false
	case 'I':
		b.moveHome()
		v.command = false
	case 'a':
		b.moveRight()
		v.command = false
	case 'A':
		b.moveEnd()
		v.command = false
	case 'x':
		e.viDelete(b.cursor, b.cursor+1)
	case 'X':
		if b.cursor > 0 {
			e.viDelete(b.cursor-1, b.cursor)
		}
	case 'w', 'W', 'e', 'E', 'b', 'B':
		viMotion(b, command, true)
	case '0':
		b.moveHome()
	case '$':
		b.moveEnd()
	case 'h':
		b.moveLeft()
	case 'l', ' ':
		b.moveRight()
	case 'j':
		e.viKey(key{kind: keyDown})
	case 'k':
		e.viKey(key{kind: keyUp})
	case 'C', 'D':
		e.viDelete(b.cursor, len(b.runes))
		v.command = command == 'D'
	case 'c', 'd', 'r':
		v.pending = command
		// c leaves command mode before its motion is typed, as busybox's does, so a motion it
		// does not take still leaves the line in insert mode.
		if command == 'c' {
			v.command = false
		}
	case 'p':
		b.moveRight()
		e.viPut()
	case 'P':
		e.viPut()
	}
}

// viOperand is the key after d, c or r.
func (e *lineEditor) viOperand(k key) {
	v, b := &e.vi, e.buffer
	operator := v.pending
	v.pending = 0
	if k.kind != keyRune {
		return
	}
	if operator == 'r' {
		if k.value >= ' ' && b.cursor < len(b.runes) {
			b.runes[b.cursor] = k.value
		}
		return
	}
	start := b.cursor
	switch motion := k.value; motion {
	case operator:
		e.viDelete(0, len(b.runes))
	case 'w', 'W', 'e', 'E':
		// dw takes the blanks after the word with it; cw leaves them, as vi's does.
		viMotion(b, motion, operator == 'd')
		if motion == 'e' || motion == 'E' {
			b.moveRight()
		}
		e.viDelete(start, b.cursor)
	case 'b', 'B':
		viMotion(b, motion, true)
		e.viDelete(b.cursor, start)
	case ' ':
		e.viDelete(start, start+1)
	case '$':
		e.viDelete(start, len(b.runes))
	}
}

// viDelete removes the runes from from to to, keeps them for p and P, and leaves the cursor
// where they began.
func (e *lineEditor) viDelete(from, to int) {
	b := e.buffer
	to = min(to, len(b.runes))
	if from < 0 || from >= to {
		return
	}
	e.vi.deleted = append([]rune(nil), b.runes[from:to]...)
	b.runes = append(b.runes[:from], b.runes[to:]...)
	b.cursor = from
}

// viPut puts back what the last deletion took, at the cursor, which ends on its last
// character.
func (e *lineEditor) viPut() {
	b, put := e.buffer, e.vi.deleted
	if len(put) == 0 {
		return
	}
	rest := append(append([]rune(nil), put...), b.runes[b.cursor:]...)
	b.runes = append(b.runes[:b.cursor], rest...)
	b.cursor += len(put) - 1
}

// viMotion moves the cursor as busybox's vi_word_motion and its five siblings move it. eat is
// whether w and W go on over the blanks after the word.
func viMotion(b *lineBuffer, motion rune, eat bool) {
	line, at := b.runes, b.cursor
	is := func(index int, class func(rune) bool) bool {
		return index >= 0 && index < len(line) && class(line[index])
	}
	notSpace := func(r rune) bool { return !unicode.IsSpace(r) }
	skip := func(class func(rune) bool, step int, limit int) {
		for at != limit && is(at+step, class) {
			at += step
		}
	}
	switch motion {
	case 'W':
		for at < len(line) && !unicode.IsSpace(line[at]) {
			at++
		}
		for eat && at < len(line) && unicode.IsSpace(line[at]) {
			at++
		}
	case 'w':
		if class := viClass(line, at); class != nil {
			skip(class, 1, len(line))
		}
		if at < len(line) {
			at++
		}
		for eat && at < len(line) && unicode.IsSpace(line[at]) {
			at++
		}
	case 'E':
		at = min(at+1, len(line))
		for at < len(line) && unicode.IsSpace(line[at]) {
			at++
		}
		skip(notSpace, 1, len(line)-1)
	case 'e':
		if at >= len(line)-1 {
			break
		}
		at++
		for at < len(line)-1 && unicode.IsSpace(line[at]) {
			at++
		}
		if class := viClass(line, at); class != nil && at < len(line)-1 {
			skip(class, 1, len(line)-1)
		}
	case 'B':
		for at > 0 && unicode.IsSpace(line[at-1]) {
			at--
		}
		for at > 0 && !unicode.IsSpace(line[at-1]) {
			at--
		}
	case 'b':
		if at <= 0 {
			break
		}
		at--
		for at > 0 && unicode.IsSpace(line[at]) {
			at--
		}
		if class := viClass(line, at); class != nil && at > 0 {
			skip(class, -1, 0)
		}
	}
	b.cursor = at
}

// viClass is the kind of word the rune at index belongs to, busybox's two: letters, digits
// and underscore, or punctuation. A blank, or the end of the line, is neither.
func viClass(line []rune, index int) func(rune) bool {
	if index < 0 || index >= len(line) {
		return nil
	}
	switch r := line[index]; {
	case isViWord(r):
		return isViWord
	case unicode.IsPunct(r) || unicode.IsSymbol(r):
		return func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }
	}
	return nil
}

func isViWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
