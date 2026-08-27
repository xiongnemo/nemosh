package applets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Whether a key press reaches its binding by every spelling a platform may send it in.
//
// `^_` did nothing on a real Windows keyboard. tcell has two input paths and they
// disagree about which Key constant a control chord is:
//
//   - a terminal posts `KeyCtrlSpace+Key(r)` for a control byte (input.go:450-452), and
//     `KeyCtrlSpace` is 64, so `0x1F` is Key(95) -- `KeyCtrlUnderscore`, as named;
//   - the Windows console has no VT screen, and for a control character whose modifier
//     mask is *exactly* Ctrl it adds 0x60 back and posts a rune instead
//     (console_win.go:725-736). Ctrl+`-` becomes 0x7F, which reads as Backspace;
//     Ctrl+Shift+`-` keeps its mask, skips the addition, and arrives as Key(31) --
//     `KeyUS`.
//
// Letters are safe by accident, because key.go:276 maps `a`-`z`+ModCtrl onto
// `KeyCtrlA+n`, the same 64-based numbering. Punctuation has no such mapping, which is
// why this binding and no other was broken.

// The arithmetic both paths depend on, pinned here so it is a test rather than a comment
// -- and so that a tcell upgrade renumbering either block fails loudly.
func TestTcell_theTwoInputPathsNumberControlKeysDifferently(t *testing.T) {
	// The terminal path: KeyCtrlSpace + the control byte.
	if got := tcell.KeyCtrlSpace + tcell.Key(0x1f); got != tcell.KeyCtrlUnderscore {
		t.Errorf("a terminal's ^_ is Key(%d); KeyCtrlUnderscore is %d",
			int(got), int(tcell.KeyCtrlUnderscore))
	}
	if got := tcell.KeyCtrlSpace + tcell.Key(0x0f); got != tcell.KeyCtrlO {
		t.Errorf("a terminal's ^O is Key(%d); KeyCtrlO is %d", int(got), int(tcell.KeyCtrlO))
	}
	// The Windows path for a chord that keeps Shift: the bare ASCII value.
	if got := tcell.Key(0x1f); got != tcell.KeyUS {
		t.Errorf("Key(0x1f) is %d; KeyUS is %d", int(got), int(tcell.KeyUS))
	}
	// And the two are genuinely different constants, which is the whole problem.
	if tcell.KeyUS == tcell.KeyCtrlUnderscore {
		t.Fatal("KeyUS and KeyCtrlUnderscore are the same; this binding needs only one of them")
	}
	// The mangling that makes Ctrl+`-` destructive on Windows rather than merely dead.
	if got := tcell.Key(0x1f + 0x60); got != tcell.KeyDEL {
		t.Errorf("0x1f+0x60 is Key(%d), want KeyDEL(%d)", int(got), int(tcell.KeyDEL))
	}
	if tcell.NewEventKey(tcell.KeyRune, 0x1f+0x60, tcell.ModCtrl).Key() != tcell.KeyBackspace {
		t.Error("Ctrl+`-` on Windows no longer reads as Backspace; the comment needs updating")
	}
}

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
		// Both constants matter, because tcell's two input paths disagree: a terminal
		// posts Key(95) = KeyCtrlUnderscore for 0x1F (input.go:452), and the Windows
		// console posts Key(31) = KeyUS for the same physical chord. The old test pressed
		// only the first, which is why the Windows half went unnoticed.
		{name: "KeyUS, what Windows sends", event: tcell.NewEventKey(tcell.KeyUS, 0, tcell.ModNone), want: editorGoToLine},
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
