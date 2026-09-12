package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Replace, and the confirm-and-step loop that was the reason it was deferred.
//
// Driven through handleKey rather than by calling the methods, because the whole point is
// that a single keystroke means something different while a confirmation is open -- `a`
// is "all" there and the letter `a` everywhere else. Calling answerReplacement directly
// would assert the easy half and skip the half that was hard.

// replaceView builds a view and runs the two prompts, leaving a confirmation open.
func replaceView(t *testing.T, text, needle, with string) *editorView {
	t.Helper()
	view := newEditorView(&editorSession{name: "nano", path: "a.txt", text: text},
		editorKeyMapFor("nano"), nil)
	view.handleKey(tcell.NewEventKey(tcell.KeyCtrlBackslash, 0, tcell.ModNone))
	if view.prompt == nil {
		t.Fatal("the replace key did not open a prompt")
	}
	typeInto(view, needle)
	view.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	typeInto(view, with)
	view.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	return view
}

func typeInto(view *editorView, text string) {
	for _, r := range text {
		view.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func answer(view *editorView, keys string) {
	for _, r := range keys {
		view.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

// `y` at every match replaces every match, and `a` does it in one keystroke.
func TestEditorReplace_yesAndAll(t *testing.T) {
	const text = "one fish\ntwo fish\nred fish\n"
	for _, test := range []struct {
		name string
		keys string
	}{
		{name: "three times yes", keys: "yyy"},
		{name: "all at once", keys: "a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := replaceView(t, text, "fish", "cat")
			answer(view, test.keys)
			if got, want := view.area.GetText(), "one cat\ntwo cat\nred cat\n"; got != want {
				t.Fatalf("the buffer is %q, want %q", got, want)
			}
			if message := view.message.GetText(true); !strings.Contains(message, "3") {
				t.Errorf("the report does not say how many: %q", message)
			}
			// The run is over, so ordinary keys reach the buffer again.
			if view.confirm != nil || view.replace != nil {
				t.Error("the replace run is still open after it finished")
			}
		})
	}
}

// `n` skips one and leaves it alone, which is the whole reason to confirm at all.
func TestEditorReplace_noSkipsOne(t *testing.T) {
	view := replaceView(t, "one fish\ntwo fish\nred fish\n", "fish", "cat")
	answer(view, "nyn")
	if got, want := view.area.GetText(), "one fish\ntwo cat\nred fish\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
	if message := view.message.GetText(true); !strings.Contains(message, "one occurrence") {
		t.Errorf("replacing one did not say so: %q", message)
	}
}

// `q` stops and keeps what was already done, which is what makes it safe to press.
func TestEditorReplace_quitKeepsWhatWasDone(t *testing.T) {
	view := replaceView(t, "a a a a\n", "a", "b")
	answer(view, "yyq")
	if got, want := view.area.GetText(), "b b a a\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
	// Escape means the same thing, since it is what cancels everything else here.
	view = replaceView(t, "a a a a\n", "a", "b")
	answer(view, "y")
	view.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if got, want := view.area.GetText(), "b a a a\n"; got != want {
		t.Fatalf("after Escape the buffer is %q, want %q", got, want)
	}
	if view.confirm != nil {
		t.Error("Escape left the confirmation open")
	}
}

// Two occurrences on one line are two separate questions, which they would not be if `n`
// skipped to the next line.
func TestEditorReplace_countsTwiceOnOneLine(t *testing.T) {
	view := replaceView(t, "x and x\n", "x", "y")
	answer(view, "ny")
	if got, want := view.area.GetText(), "x and y\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
}

// Replacing a string with one that contains it terminates. `a` -> `aa` would otherwise
// find the second `a` it had just written, for as long as the buffer held out.
func TestEditorReplace_doesNotRewriteWhatItJustWrote(t *testing.T) {
	view := replaceView(t, "a a\n", "a", "aa")
	answer(view, "a")
	if got, want := view.area.GetText(), "aa aa\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
}

// Replacing with nothing deletes, which is a thing people want rather than a mistake.
func TestEditorReplace_withNothingDeletes(t *testing.T) {
	view := replaceView(t, "keep XX this XX\n", "XX ", "")
	answer(view, "a")
	if got, want := view.area.GetText(), "keep this XX\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
}

// Nothing found says so and changes nothing.
func TestEditorReplace_reportsWhatItCannotFind(t *testing.T) {
	view := replaceView(t, "hello\n", "zzznotthere", "x")
	if got, want := view.area.GetText(), "hello\n"; got != want {
		t.Fatalf("the buffer changed: %q", got)
	}
	if message := view.message.GetText(true); !strings.Contains(message, "not found") {
		t.Errorf("a failed replace did not say so: %q", message)
	}
	if view.confirm != nil {
		t.Error("a failed replace left a confirmation open")
	}
}

// An unrecognised key asks again rather than guessing. Guessing `n` would be safe and
// guessing `y` would not, and a prompt that treats every stray key as "no" is one people
// learn to distrust.
func TestEditorReplace_anUnknownAnswerAsksAgain(t *testing.T) {
	view := replaceView(t, "a a\n", "a", "b")
	answer(view, "z")
	if view.confirm == nil {
		t.Fatal("an unrecognised answer closed the confirmation")
	}
	if got := view.area.GetText(); got != "a a\n" {
		t.Fatalf("an unrecognised answer changed the buffer: %q", got)
	}
	// And the real answer still works afterwards.
	answer(view, "a")
	if got, want := view.area.GetText(), "b b\n"; got != want {
		t.Fatalf("the buffer is %q, want %q", got, want)
	}
}

// A read-only buffer refuses before asking anything, rather than asking and then failing.
func TestEditorReplace_refusesOnAReadOnlyBuffer(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.txt", text: "a\n", readOnly: true},
		editorKeyMapFor("nano"), nil)
	view.handleKey(tcell.NewEventKey(tcell.KeyCtrlBackslash, 0, tcell.ModNone))
	if view.prompt != nil {
		t.Fatal("a read-only buffer opened a replace prompt")
	}
	if message := view.message.GetText(true); !strings.Contains(message, "Read-only") {
		t.Errorf("the refusal does not say why: %q", message)
	}
}

// While a confirmation is open the action keys are answers, not actions. Otherwise
// answering a replace would quit the editor, which is the failure this ordering prevents.
func TestEditorReplace_confirmationOwnsEveryKey(t *testing.T) {
	view := replaceView(t, "x x\n", "x", "y")
	// ^X is quit in nano. With a confirmation open it must not quit, and since the
	// view has no application a quit would panic -- so surviving is the assertion.
	view.handleKey(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModNone))
	// It was not a y/n/a/q, so the confirmation is still open and nothing changed.
	if view.confirm == nil {
		t.Fatal("a binding key closed the confirmation")
	}
	if got := view.area.GetText(); got != "x x\n" {
		t.Fatalf("the buffer changed: %q", got)
	}
}

// Both names reach replace, by their own key.
func TestEditorReplace_bothNamesBindIt(t *testing.T) {
	for _, test := range []struct {
		name string
		key  tcell.Key
	}{
		{name: "nano", key: tcell.KeyCtrlBackslash},
		{name: "micro", key: tcell.KeyCtrlR},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := editorKeyMapFor(test.name).lookup(
				tcell.NewEventKey(test.key, 0, tcell.ModNone)); got != editorReplace {
				t.Fatalf("%s: the replace key resolves to %v", test.name, got)
			}
		})
	}
	// And nano's punctuation chord answers to what Windows sends for it, which is the
	// lesson `^_` cost two commits to learn: 0x1C + 0x60 is `|`.
	nano := editorKeyMapFor("nano")
	for _, event := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyFS, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, '|', tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyRune, '\\', tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt),
	} {
		if got := nano.lookup(event); got != editorReplace {
			t.Errorf("nano: %s resolves to %v, want replace", event.Name(), got)
		}
	}
}

// The buffer keeps its terminating newline through a rewrite.
//
// It did not: editorLines drops the empty element a trailing newline produces, correctly,
// and strings.Join cannot know it was ever there -- so cut and paste silently deleted the
// file's final newline and `^O` wrote it back a byte short. Found while writing replace,
// which rebuilds the buffer the same way and would have inherited it.
func TestEditorView_rewritingTheBufferKeepsTheFinalNewline(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		do   func(*editorView)
		want string
	}{
		{
			name: "cut", text: "one\ntwo\nthree\n",
			do:   func(v *editorView) { v.handleKey(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone)) },
			want: "two\nthree\n",
		},
		{
			name: "cut then paste", text: "one\ntwo\n",
			do: func(v *editorView) {
				v.handleKey(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
				v.handleKey(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
			},
			want: "one\ntwo\n",
		},
		{
			// A file with no final newline must not gain one either.
			name: "no newline to begin with", text: "one\ntwo",
			do:   func(v *editorView) { v.handleKey(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone)) },
			want: "two",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := newEditorView(&editorSession{name: "nano", path: "a.txt", text: test.text},
				editorKeyMapFor("nano"), nil)
			test.do(view)
			if got := view.area.GetText(); got != test.want {
				t.Fatalf("the buffer is %q, want %q", got, test.want)
			}
		})
	}
	// And replace, which is why this was found.
	view := replaceView(t, "one fish\ntwo fish\n", "fish", "cat")
	answer(view, "a")
	if got, want := view.area.GetText(), "one cat\ntwo cat\n"; got != want {
		t.Fatalf("after replace the buffer is %q, want %q", got, want)
	}
}
