package applets

import "strings"

// The dot command, busybox's: the keys of the last command that changed the buffer are kept,
// from the command's own key to its end -- an insert's Escape -- and . types them again, with
// the count it was given or the one typed before the dot, which it then keeps. The whole of it
// is one change to undo.

// viRepeatable is busybox's list of the commands a dot repeats, and vim's S.
const viRepeatable = "aAcCdDiIJoOpPrRsSxX<>~"

// startRecording begins keeping keys at a command that changes the buffer.
func (e *viEditor) startRecording(key rune) {
	if e.repeating {
		return
	}
	if e.recording == nil && e.mode == viNormal && e.pending == 0 && e.op == 0 && key != 0 &&
		strings.ContainsRune(viRepeatable, key) {
		e.recording, e.recordCount = []rune{}, e.count
	}
	if e.recording != nil {
		e.recording = append(e.recording, key)
	}
}

// finishRecording keeps the command's keys once it is done.
func (e *viEditor) finishRecording() {
	if e.recording == nil || e.repeating || e.mode != viNormal || e.pending != 0 || e.op != 0 || e.prefix {
		return
	}
	e.lastChange, e.lastCount, e.recording = e.recording, e.recordCount, nil
}

// repeatChange is the dot.
func (e *viEditor) repeatChange() {
	if len(e.lastChange) == 0 {
		return
	}
	if e.count > 0 {
		e.lastCount = e.count
	}
	e.closeChange()
	e.repeating = true
	e.count = e.lastCount
	for _, key := range e.lastChange {
		e.key(key)
	}
	e.repeating = false
	e.closeChange()
}
