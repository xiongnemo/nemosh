package main

import "slices"

// Undo, readline's: `C-_` and `C-x C-u` take back the last change to the line, and pressed
// again the one before, back to the line as it was first drawn; `M-r`, revert-line, takes
// them all back at once. Characters typed one after another are one change, up to twenty
// of them, as readline joins them; a key that moves the cursor is no change at all. A line
// recalled from history starts its own, as readline keeps each line's undo with the line.
// busybox's editor has undo only in vi mode, `u`, which this one has too.

// undoTypingRun is how many typed characters readline's rl_insert_text joins into one undo.
const undoTypingRun = 20

// lineUndo is what the line was before each change, oldest first.
type lineUndo struct {
	states []lineState
	// typing is how many characters the newest change typed, and typedTo where the cursor
	// stood after the last of them, for the next to join it.
	typing  int
	typedTo int
}

// lineState is a line and its cursor.
type lineState struct {
	runes  []rune
	cursor int
}

func (b *lineBuffer) state() lineState {
	return lineState{runes: append([]rune(nil), b.runes...), cursor: b.cursor}
}

func (b *lineBuffer) restore(state lineState) {
	b.runes, b.cursor = state.runes, state.cursor
}

// note is a key's mark on the undo: before, the line as it was, kept when the key changed
// it, unless it typed a character straight after a run of typed ones.
func (u *lineUndo) note(before lineState, typed bool, b *lineBuffer) {
	if slices.Equal(before.runes, b.runes) {
		u.typing = 0
		return
	}
	if typed && u.typing > 0 && u.typing < undoTypingRun && before.cursor == u.typedTo {
		u.typing++
		u.typedTo = b.cursor
		return
	}
	u.states = append(u.states, before)
	u.typing = 0
	if typed {
		u.typing, u.typedTo = 1, b.cursor
	}
}

// undo takes back the newest change, and reports whether there was one.
func (u *lineUndo) undo(b *lineBuffer) bool {
	if len(u.states) == 0 {
		return false
	}
	b.restore(u.states[len(u.states)-1])
	u.states = u.states[:len(u.states)-1]
	u.typing = 0
	return true
}

// revert takes back every change.
func (u *lineUndo) revert(b *lineBuffer) {
	if len(u.states) > 0 {
		b.restore(u.states[0])
	}
	u.clear()
}

func (u *lineUndo) clear() { u.states, u.typing = nil, 0 }

// beforeKey settles what a key ends before it is handled -- yank-last-arg's repeat, a run of
// kills, a yank M-y can turn -- and answers whether the key is a kill, and the line as it
// stands, for afterKey.
func (e *lineEditor) beforeKey(k key) (bool, lineState) {
	if k.kind != keyYankLastArg {
		e.lastArg.active = false
	}
	killing := k.kind == keyClearLine || k.kind == keyKillToEnd || k.kind == keyDeleteWord || k.kind == keyDeleteWordForward
	if !killing {
		e.kills.joining = false
	}
	if k.kind != keyYank && k.kind != keyYankPop {
		e.kills.yanked.active = false
	}
	return killing, e.buffer.state()
}

// afterKey is a key's mark on the kill ring and on the undo, once it is handled.
func (e *lineEditor) afterKey(k key, killing bool, before lineState) {
	e.kills.joining = killing
	switch k.kind {
	case keyUndo, keyRevertLine:
	case keyUp, keyDown, keyHistoryPrefixBackward, keyHistoryPrefixForward:
		e.undo.clear()
	default:
		e.undo.note(before, k.kind == keyRune, e.buffer)
	}
}
