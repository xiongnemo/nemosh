package applets

// Undo and redo, vim's: u takes back a change at a time, as many as there were, ^R does it again,
// and a change is a command -- an insert from where it began to its Escape, an arrow inside it
// beginning another. busybox has the same u but no redo, its ^R redrawing the screen, and undoes
// a cw, an o or a p in two steps. U puts back the line the last changes were made on, and U
// again takes that back, vim's; busybox's took the line anew whenever the cursor came to it.

// viEdit is one replaceLines: lines [from, from+len(before)) were before, and became after.
type viEdit struct {
	from          int
	before, after [][]rune
}

// viChange is a command's edits, the cursor where they began, and a number that says which
// state of the buffer it leaves.
type viChange struct {
	edits  []viEdit
	cursor viPos
	seq    int
}

// record keeps an edit in the change the command is making. A run of edits to the same lines, as
// typing makes, is kept as one.
func (e *viEditor) record(from int, before, after [][]rune) {
	if e.undoing {
		return
	}
	if !e.changeOpen {
		e.seq++
		e.undo = append(e.undo, viChange{cursor: e.cursor(), seq: e.seq})
		e.redo, e.changeOpen = nil, true
	}
	change := &e.undo[len(e.undo)-1]
	edit := viEdit{from: from, before: before, after: after}
	if count := len(change.edits); count > 0 {
		last := change.edits[count-1]
		if from >= last.from && from+len(before) <= last.from+len(last.after) {
			offset := from - last.from
			merged := append(append(append([][]rune(nil), last.after[:offset]...), after...), last.after[offset+len(before):]...)
			edit = viEdit{from: last.from, before: last.before, after: merged}
			change.edits = change.edits[:count-1]
		}
	}
	change.edits = append(change.edits, edit)
	e.noteLineChange(from, before, after)
}

// closeChange ends the change being made; the next edit begins another.
func (e *viEditor) closeChange() { e.changeOpen = false }

// undoChange is u.
func (e *viEditor) undoChange() {
	e.closeChange()
	if len(e.undo) == 0 {
		e.say(false, "Already at oldest change")
		return
	}
	change := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.undoing = true
	for index := len(change.edits) - 1; index >= 0; index-- {
		edit := change.edits[index]
		e.replaceLines(edit.from, edit.from+len(edit.after), copyLines(edit.before))
	}
	e.undoing = false
	e.redo = append(e.redo, change)
	e.cursorAfter(change, true)
}

// redoChange is ^R.
func (e *viEditor) redoChange() {
	e.closeChange()
	if len(e.redo) == 0 {
		e.say(false, "Already at newest change")
		return
	}
	change := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.undoing = true
	for _, edit := range change.edits {
		e.replaceLines(edit.from, edit.from+len(edit.before), copyLines(edit.after))
	}
	e.undoing = false
	e.undo = append(e.undo, change)
	e.cursorAfter(change, false)
}

// cursorAfter is vim's u_undoredo: the first line the change touched, or the one above where
// undoing it only took lines away, at the column the change began in if that is its line.
func (e *viEditor) cursorAfter(change viChange, undoing bool) {
	first := change.edits[0]
	for _, edit := range change.edits {
		if edit.from < first.from {
			first = edit
		}
	}
	row, gone := first.from, len(first.before) == 0
	if !undoing {
		gone = len(first.after) == 0
	}
	if gone {
		row--
	}
	e.row = max(0, min(row, len(e.lines)-1))
	e.col = e.firstNonBlank(e.row)
	if e.row == change.cursor.row {
		e.col = change.cursor.col
	}
}

// isModified says the buffer is not what was last written, or read.
func (e *viEditor) isModified() bool {
	current := 0
	if len(e.undo) > 0 {
		current = e.undo[len(e.undo)-1].seq
	}
	return current != e.savedSeq
}

// markSaved records that the buffer is what the file now holds.
func (e *viEditor) markSaved() {
	e.closeChange()
	e.savedSeq = 0
	if len(e.undo) > 0 {
		e.savedSeq = e.undo[len(e.undo)-1].seq
	}
}

// noteLineChange keeps what U puts back: the line as it was before the changes made to it since
// the cursor last changed another.
func (e *viEditor) noteLineChange(from int, before, after [][]rune) {
	if e.restoringLine {
		return
	}
	if len(before) != 1 || len(after) != 1 {
		e.lineBefore = nil
		return
	}
	if e.lineBefore == nil || e.lineRow != from {
		e.lineRow, e.lineBefore = from, append([]rune(nil), before[0]...)
	}
}

// restoreLine is U.
func (e *viEditor) restoreLine() {
	if e.lineBefore == nil || e.lineRow >= len(e.lines) {
		e.beep()
		return
	}
	now := append([]rune(nil), e.lines[e.lineRow]...)
	e.restoringLine = true
	e.replaceLines(e.lineRow, e.lineRow+1, [][]rune{e.lineBefore})
	e.restoringLine = false
	e.lineBefore, e.row = now, e.lineRow
}

func copyLines(lines [][]rune) [][]rune {
	copied := make([][]rune, len(lines))
	for index, line := range lines {
		copied[index] = append([]rune(nil), line...)
	}
	return copied
}
