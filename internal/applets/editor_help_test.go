package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// The help panel, the key reader, and the Escape prefix.
//
// All three come from the same report: `^_` did nothing, `^/` did nothing either, and
// `^-` was eaten by the terminal before the editor saw it. Two guesses at what the
// console sends were both wrong, which is the argument for a terminal that can be asked
// rather than guessed at.

func helpPanelFor(t *testing.T, name string) string {
	t.Helper()
	view := newEditorView(&editorSession{name: name, path: "a.go", text: "x\n"},
		editorKeyMapFor(name), nil)
	return view.helpPanel("")
}

// The panel says what is *not* there, which is the reason it is a panel rather than a
// row: the keys are already on screen in the legend, so what a legend owes the reader is
// the rest.
func TestEditorHelpPanel_saysWhatIsNotThere(t *testing.T) {
	panel := helpPanelFor(t, "nano")
	for _, absent := range []string{"multiple buffers", "replace", "mouse", "soft wrap"} {
		if !strings.Contains(panel, absent) {
			t.Errorf("the help panel does not mention %q", absent)
		}
	}
	// And it explains *why* rather than only listing, which is what makes it a legend.
	if !strings.Contains(panel, "line-start table") {
		t.Error("the panel says soft wrap is absent without saying why")
	}
}

// Every binding is described, walked from the table so the panel cannot claim a key the
// editor does not bind -- the same property `-H` has.
func TestEditorHelpPanel_describesEveryBinding(t *testing.T) {
	for _, name := range []string{"nano", "micro"} {
		panel := helpPanelFor(t, name)
		for _, binding := range editorKeyMapFor(name).bindings {
			if !strings.Contains(panel, binding.label) {
				t.Errorf("%s: the panel omits the label %q", name, binding.label)
			}
			if !strings.Contains(panel, binding.describes) {
				t.Errorf("%s: the panel omits %q", name, binding.describes)
			}
		}
		// The other name's keys are nowhere in it.
		other := "^S Save"
		if name == "micro" {
			other = "^O Write Out"
		}
		if strings.Contains(panel, other) {
			t.Errorf("%s's panel mentions %q, which belongs to the other name", name, other)
		}
	}
}

// The key reader names a press in the terms the binding table matches on.
//
// The two spellings look identical in tcell's own Name(), which is exactly the
// distinction that mattered: on Windows a Ctrl'd letter arrives as a rune with ModCtrl
// rather than as a Key constant, and a reader that printed only Name() would not have
// told the two apart.
func TestDescribeKeyEvent_distinguishesTheTwoSpellings(t *testing.T) {
	// A *letter* cannot be used to show the difference: tcell normalises `a`-`z` with
	// ModCtrl back into the KeyCtrl* constants (key.go:276), so both spellings of Ctrl-O
	// really are the same event. Punctuation gets no such mapping and stays a rune, which
	// is exactly why `^_` behaved differently from `^O` and why this test uses `/`.
	asKey := describeKeyEvent(tcell.NewEventKey(tcell.KeyUS, 0, tcell.ModNone))
	asRune := describeKeyEvent(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModCtrl))
	if asKey == asRune {
		t.Fatalf("a Key and a rune describe identically: %q", asKey)
	}
	if !strings.Contains(asKey, "key constant 31") {
		t.Errorf("the Key spelling does not report its constant: %q", asKey)
	}
	if !strings.Contains(asRune, "rune") || !strings.Contains(asRune, "U+002F") {
		t.Errorf("the rune spelling does not report the rune: %q", asRune)
	}
	if !strings.Contains(asRune, "Ctrl") {
		t.Errorf("the rune spelling does not report the modifier: %q", asRune)
	}
	// A modifierless key says so rather than leaving the field blank, because "no
	// modifiers arrived" is itself the answer in the case this exists for.
	plain := describeKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if !strings.Contains(plain, "modifiers none") {
		t.Errorf("a plain key does not say it had no modifiers: %q", plain)
	}
}

// Escape then a letter is the meta chord, which is what `M-G` means on a terminal and
// the one spelling of it that cannot fail to arrive: both halves are ordinary keys.
// Alt and a letter is not -- the Windows console reports no character for it and tcell
// drops a key event with no character and no recognised virtual key.
func TestEditorView_escapeThenLetterIsAMetaChord(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "one\ntwo\nthree\n"},
		editorKeyMapFor("nano"), nil)

	// Escape alone arms the prefix and shows that it did, because a prefix that waits
	// silently is indistinguishable from a key that did nothing.
	if got := view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Fatal("Escape reached the text area instead of arming the prefix")
	}
	if !view.meta {
		t.Fatal("Escape did not arm the meta prefix")
	}
	if !strings.Contains(view.message.GetText(true), "M-") {
		t.Errorf("the prefix is not shown: %q", view.message.GetText(true))
	}

	// Then G opens the go-to-line prompt, which is what M-G means in nano.
	if got := view.handleKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone)); got != nil {
		t.Fatal("the letter after Escape reached the text area")
	}
	if view.meta {
		t.Error("the prefix stayed armed after being used")
	}
	if view.prompt == nil {
		t.Fatal("Escape then G did not open the go-to-line prompt")
	}
	if !strings.Contains(view.message.GetText(true), "Go to line") {
		t.Errorf("the prompt is not the go-to-line one: %q", view.message.GetText(true))
	}
}

// A letter that means nothing after Escape says so and does not silently insert itself,
// which would otherwise put a stray character in the buffer.
func TestEditorView_anUnboundMetaChordSaysSo(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "x\n"},
		editorKeyMapFor("nano"), nil)
	view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if got := view.handleKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)); got != nil {
		t.Fatal("an unbound meta chord passed its letter to the buffer")
	}
	if message := view.message.GetText(true); !strings.Contains(message, "M-J") {
		t.Errorf("an unbound meta chord did not name itself: %q", message)
	}
	if view.meta {
		t.Error("the prefix stayed armed")
	}
}

// Escape twice types nothing and clears the prefix, which is the way out of one pressed
// by accident.
func TestEditorView_escapeTwiceClearsThePrefix(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "x\n"},
		editorKeyMapFor("nano"), nil)
	view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if view.meta {
		t.Fatal("Escape twice left the prefix armed")
	}
	if message := view.message.GetText(true); strings.Contains(message, "M-") {
		t.Errorf("Escape twice left the prefix on the message row: %q", message)
	}
}

// A prompt still owns Escape: cancelling a search must not arm a meta prefix instead.
func TestEditorView_promptKeepsEscape(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "x\n"},
		editorKeyMapFor("nano"), nil)
	view.handleKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if view.prompt == nil {
		t.Fatal("^W did not open the search prompt")
	}
	view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if view.prompt != nil {
		t.Error("Escape did not cancel the prompt")
	}
	if view.meta {
		t.Error("Escape cancelled the prompt and armed a meta prefix as well")
	}
}

// Ordinary typing is untouched by the prefix machinery, which is the regression worth
// guarding: every key press now goes through it.
func TestEditorView_ordinaryKeysStillReachTheBuffer(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: ""},
		editorKeyMapFor("nano"), nil)
	for _, event := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone),
	} {
		if got := view.handleKey(event); got != event {
			t.Errorf("%v was swallowed instead of reaching the text area", event.Name())
		}
	}
}
