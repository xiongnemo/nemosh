package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// -R is a viewer: the buffer cannot change, and every key that only moves still moves. Only
// saving was refused. nano's ^K and ^U cut and pasted lines into a read-only buffer, and a key
// the text area binds for itself -- ^D, ^K, ^W, ^U, ^X, ^V, ^Z, ^Y, Alt-Backspace -- went
// through to it, since the read-only check knew only typing, Enter, Tab, Backspace and Delete.
func TestEditor_readOnlyBufferCannotChange(t *testing.T) {
	newView := func() *editorView {
		return newEditorView(&editorSession{name: "nano", path: "a.txt", text: "one\ntwo\n", readOnly: true},
			editorKeyMapFor("nano"), nil)
	}

	t.Run("nano's cut and paste", func(t *testing.T) {
		for _, key := range []tcell.Key{tcell.KeyCtrlK, tcell.KeyCtrlU} {
			view := newView()
			view.cut = "zero"
			view.handleKey(tcell.NewEventKey(key, 0, tcell.ModCtrl))
			if got := view.area.GetText(); got != "one\ntwo\n" {
				t.Errorf("after %s the buffer is %q", tcell.KeyNames[key], got)
			}
			if message := view.message.GetText(true); !strings.Contains(message, "Read-only") {
				t.Errorf("%s: the refusal does not say why: %q", tcell.KeyNames[key], message)
			}
		}
	})

	t.Run("the text area's own editing keys", func(t *testing.T) {
		view := newView()
		for _, event := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModAlt),
		} {
			if view.handleKey(event) != nil {
				t.Errorf("%s reached the text area", event.Name())
			}
		}
	})

	t.Run("moving still moves", func(t *testing.T) {
		view := newView()
		for _, event := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt),
		} {
			if view.handleKey(event) == nil {
				t.Errorf("%s was refused", event.Name())
			}
		}
	})
}
