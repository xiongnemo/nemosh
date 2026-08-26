package applets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Whether a key press reaches its binding by either spelling.
//
// `^_` did nothing on a real Windows keyboard, and reading tcell explains why: there
// is no VT screen on Windows, so input goes through the console API, and for a control
// character with Ctrl held tcell adds 0x60 back and posts a *rune with ModCtrl* rather
// than a Key constant (console_win.go:725-736). Which spelling arrives for a given
// physical key is a property of the console, not something this code can know -- so it
// accepts both.

func TestEditorKeyMap_acceptsEitherSpellingOfAKey(t *testing.T) {
	nano := editorKeyMapFor("nano")
	for _, test := range []struct {
		name  string
		event *tcell.EventKey
		want  editorAction
	}{
		// The Key constant, which is what a terminal sends.
		{name: "KeyCtrlO", event: tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModNone), want: editorSave},
		// The modified rune, which is what the Windows console sends for the same press.
		{name: "Ctrl-o as a rune", event: tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModCtrl), want: editorSave},
		// Upper case, because Ctrl-Shift-O is still Ctrl-O.
		{name: "Ctrl-O upper", event: tcell.NewEventKey(tcell.KeyRune, 'O', tcell.ModCtrl), want: editorSave},

		// Go to line, the binding that was unreachable.
		//
		// KeyUS is the one that matters: it is what a terminal's `^_` (0x1F) actually
		// becomes. `KeyCtrlUnderscore` is 95 and *nothing produces 95* -- the old test
		// passed by synthesising it, which no keyboard can do. It stays accepted in case
		// some driver does emit it, but this is the case that would have caught the bug.
		{name: "KeyUS, which is what ^_ really is", event: tcell.NewEventKey(tcell.KeyUS, 0, tcell.ModNone), want: editorGoToLine},
		{name: "KeyCtrlUnderscore", event: tcell.NewEventKey(tcell.KeyCtrlUnderscore, 0, tcell.ModNone), want: editorGoToLine},
		{name: "Ctrl-slash", event: tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModCtrl), want: editorGoToLine},
		// `_` needs Shift on this keyboard, so Shift must not disqualify it -- that is a
		// fact about the layout rather than about the binding.
		{name: "Ctrl-Shift-underscore", event: tcell.NewEventKey(tcell.KeyRune, '_', tcell.ModCtrl|tcell.ModShift), want: editorGoToLine},
		{name: "Alt-g", event: tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModAlt), want: editorGoToLine},

		// And what must NOT be an action: a plain letter is text to insert.
		{name: "plain o", event: tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone), want: editorNothing},
		{name: "plain g", event: tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone), want: editorNothing},
		{name: "plain slash", event: tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone), want: editorNothing},
		// Alt and a letter that is only bound under Ctrl is not that binding: the
		// modifier has to match, or Alt-o would silently save.
		{name: "Alt-o", event: tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt), want: editorNothing},
		// A letter nothing binds.
		{name: "Ctrl-z", event: tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModCtrl), want: editorNothing},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := nano.lookup(test.event); got != test.want {
				t.Fatalf("lookup(%s) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

// Ctrl-z reaches the text area rather than the editor, which is what makes tview's own
// undo work -- and undo working is why `-H` does not list it as absent. Asserted here
// because the feature list would otherwise be lying on the strength of a key nothing
// tests.
func TestEditorKeyMap_leavesUndoToTheTextArea(t *testing.T) {
	for _, name := range []string{"nano", "micro"} {
		keys := editorKeyMapFor(name)
		for _, event := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModCtrl),
			tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModCtrl),
		} {
			if got := keys.lookup(event); got != editorNothing {
				t.Errorf("%s binds %v to %v, which shadows the text area's undo", name, event.Name(), got)
			}
		}
	}
}

// Every binding is reachable by both spellings, checked by walking the table rather
// than by listing keys -- so a binding added later without a chord fails here.
func TestEditorKeyMap_everyBindingHasARuneSpelling(t *testing.T) {
	for _, name := range []string{"nano", "micro"} {
		for _, binding := range editorKeyMapFor(name).bindings {
			if len(binding.chords) == 0 {
				t.Errorf("%s: %q has no rune spelling, so it is unreachable where the console sends one",
					name, binding.label)
				continue
			}
			for _, chord := range binding.chords {
				event := tcell.NewEventKey(tcell.KeyRune, chord.rune, chord.mods)
				if got := editorKeyMapFor(name).lookup(event); got != binding.action {
					t.Errorf("%s: %q chord %c+%v resolves to %v, want %v",
						name, binding.label, chord.rune, chord.mods, got, binding.action)
				}
			}
		}
	}
}
