package main

// Walking through history, which the arrows and Page Up/Down share.
//
// **The line being typed is saved when a walk leaves it, and handed back unchanged when a
// walk returns to it.** That is the whole of the design, and both keys that walk used to get
// it wrong in the same way.
//
// Down replaced the typed line with an empty one on the way back, so a half-typed command
// was lost by pressing Up and then Down. Page Up then inherited the fault in a worse form:
// it took its prefix from the *current* buffer on every press, so after Up had recalled a
// whole line the prefix was that entire line -- and coming back put it on the line as though
// it had been typed. A key log from a terminal showed the result: Up, Page Up, Page Down,
// then `exit`, and the shell ran `git pushexit`.
//
// Saving the typed line once, when the walk begins, fixes all three, because it means a walk
// can only ever hand back what was typed. Nothing recalled survives it onto the line.

// beginHistoryWalk saves the typed line as a walk leaves it.
//
// Only the first step saves. A walk already in progress has the typed line already, and the
// buffer now holds a history entry that must not overwrite it.
func (e *lineEditor) beginHistoryWalk() {
	if e.recall != 0 {
		return
	}
	e.typed = e.buffer.String()
	e.typedCursor = e.buffer.cursor
}

// showHistory puts the entry at target on the line, counted from the end as recall is, or
// hands the typed line back at zero with its cursor where it was left.
func (e *lineEditor) showHistory(target int) {
	e.recall = target
	if target == 0 {
		e.buffer.replace(e.typed)
		e.buffer.cursor = e.typedCursor
		return
	}
	e.buffer.replace(e.history[len(e.history)-target])
}

// recallHistory walks through everything, one entry at a time. Direction is +1 for older
// and -1 for newer; walking past the newest hands the typed line back.
func (e *lineEditor) recallHistory(direction int) {
	target := e.recall + direction
	if target < 0 || target > len(e.history) {
		return
	}
	e.beginHistoryWalk()
	e.showHistory(target)
}
