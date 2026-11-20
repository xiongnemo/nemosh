package applets

import "github.com/gdamore/tcell/v2"

// -R is a viewer. Saving was the only thing it refused: nano's own ^K and ^U cut and pasted
// lines into a read-only buffer, and the keys the text area binds for itself -- ^D, ^K, ^W, ^U,
// ^X, ^V, ^Z, ^Y, Alt-Backspace -- reached it, because the check asked which keys edit and
// knew only typing, Enter, Tab, Backspace and Delete. It asks which keys only look now.

const readOnlyMessage = "[red]Read-only: the buffer cannot be changed[-]"

// refusesReadOnly says why and reports true when the buffer is read-only, for an action that
// would change it.
func (v *editorView) refusesReadOnly() bool {
	if v.session.readOnly {
		v.setMessage(readOnlyMessage)
	}
	return v.session.readOnly
}

// isViewingKey reports whether a key the text area would be given only moves the cursor,
// scrolls or selects (tview's TextArea documents them), which is all read-only lets through.
func isViewingKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight, tcell.KeyHome, tcell.KeyEnd,
		tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyCtrlA, tcell.KeyCtrlE, tcell.KeyCtrlF, tcell.KeyCtrlB,
		tcell.KeyCtrlL, tcell.KeyCtrlQ:
		return true
	case tcell.KeyRune:
		// Alt-B and Alt-F are its word motions.
		return event.Modifiers()&tcell.ModAlt != 0 && (event.Rune() == 'b' || event.Rune() == 'f')
	}
	return false
}
