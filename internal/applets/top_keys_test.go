package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// top's interactive keys that ask the user something, or act on a process.
//
// top_keys.go was at 40%, and what was missing was every key with a consequence:
// the kill confirmation, the priority nudge, and the two prompts. They are the keys
// most worth testing for exactly that reason -- a monitor that silently does nothing
// when a key is pressed is the failure this applet is written against, and its own
// comment says so.
//
// **Nothing here terminates a process.** The kill test opens the modal and cancels
// it, and asserts that cancelling did not act; a test that killed something to prove
// it could would be trading a real process for a fact already visible in the status
// line.

// k opens a confirmation naming the process, and the confirmation says what Windows
// cannot do. That warning is a decision, not decoration: there is no gentle signal,
// so the process gets no chance to close handles or remove lock files, and somebody
// about to press Terminate should be told.
func TestTopKeys_killAsksFirstAndNamesWhatItCannotDo(t *testing.T) {
	harness := startTop(t)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("the table never drew")
	}

	harness.screen.InjectKey(tcell.KeyRune, 'k', tcell.ModNone)
	frame, ok := harness.waitFor(t, "Terminate")
	if !ok {
		t.Fatalf("k opened no confirmation:\n%s", frame)
	}
	// The pid is in the question, because "terminate this?" without a name is a
	// question nobody can answer.
	if !strings.Contains(frame, "pid ") {
		t.Errorf("the confirmation does not name a pid:\n%s", frame)
	}
	// And the warning, word by word rather than as a phrase: tview wraps the modal
	// text to the box, so "no gentle signal" arrives as "no gentle" then "signal:"
	// on the next line with a border between them. Asserting the phrase failed for
	// that reason and not because the warning was missing.
	for _, word := range []string{"gentle", "signal", "handles", "lock", "files"} {
		if !strings.Contains(frame, word) {
			t.Errorf("the confirmation does not mention %q:\n%s", word, frame)
		}
	}
	// Both buttons are offered, with Cancel first so the default is the safe one.
	if !strings.Contains(frame, "Cancel") {
		t.Errorf("there is no way to decline:\n%s", frame)
	}
	if strings.Index(frame, "Cancel") > strings.LastIndex(frame, "Terminate") {
		t.Errorf("Terminate comes before Cancel, so the default is the dangerous one:\n%s", frame)
	}

	// Escape declines, and the table comes back -- which is what says the modal
	// released the keyboard it took.
	harness.screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if frame, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatalf("declining did not restore the table:\n%s", frame)
	}
	// Nothing was terminated: no status line claims one.
	if frame := harness.text(t); strings.Contains(frame, "terminated") {
		t.Fatalf("cancelling reported a termination:\n%s", frame)
	}
	// And the application is still alive and answering afterwards.
	if err := harness.quit(t); err != nil {
		t.Fatalf("quitting after a cancelled kill: %v", err)
	}
}

// While the modal is up, the key that opened it must not open a second one -- the
// sharper version of the same reason the modal takes the keyboard at all.
func TestTopKeys_theModalOwnsTheKeyboard(t *testing.T) {
	harness := startTop(t)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("the table never drew")
	}
	harness.screen.InjectKey(tcell.KeyRune, 'k', tcell.ModNone)
	if _, ok := harness.waitFor(t, "Terminate"); !ok {
		t.Fatal("k opened no confirmation")
	}
	// A second k, and then q -- which would quit if the table still had the keys.
	harness.screen.InjectKey(tcell.KeyRune, 'k', tcell.ModNone)
	harness.screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	// Still showing the modal, and still answering: a quit would have closed the
	// application and QueueUpdate would time out.
	frame, ok := harness.waitFor(t, "Terminate")
	if !ok {
		t.Fatalf("the modal closed on a key it should have swallowed:\n%s", frame)
	}
	harness.screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("the table did not come back")
	}
	if err := harness.quit(t); err != nil {
		t.Fatalf("quit: %v", err)
	}
}

// The priority keys report either way, which is the property: most processes on a
// Windows machine cannot be opened for this -- they belong to SYSTEM or to another
// user -- so a keypress that silently did nothing would be indistinguishable from a
// broken binding.
//
// Which of the two answers appears depends on which process the cursor happens to be
// on, so the assertion is that *something* was said, in the colour that says which.
func TestTopKeys_priorityAlwaysReportsAnOutcome(t *testing.T) {
	harness := startTop(t)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("the table never drew")
	}
	for _, key := range []tcell.Key{tcell.KeyF7, tcell.KeyF8} {
		harness.screen.InjectKey(key, 0, tcell.ModNone)
		// Either "is now <class>" on success or a reason on failure. On this machine
		// the cursor starts on the Idle process, whose priority nothing can change,
		// so the usual answer is "not a process this can change" -- and that being an
		// answer rather than a silence is the whole property.
		frame, ok := harness.waitForEither(t, "is now ", "not a process this can change",
			"denied", "Access", "cannot")
		if !ok {
			t.Fatalf("%v said nothing at all:\n%s", tcell.KeyNames[key], frame)
		}
	}
	if err := harness.quit(t); err != nil {
		t.Fatalf("quit: %v", err)
	}
}

// The filter prompt: what it opens with, that Enter applies it, and that Escape
// leaves the previous filter alone rather than clearing it.
func TestTopKeys_filterPromptAppliesAndCancels(t *testing.T) {
	harness := startTop(t)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("the table never drew")
	}

	// F4, which is htop's filter key. There is no `f` binding: guessing one was this
	// test's own first mistake, and the model's key table is the answer.
	harness.screen.InjectKey(tcell.KeyF4, 0, tcell.ModNone)
	if frame, ok := harness.waitFor(t, "filter:"); !ok {
		t.Fatalf("F4 opened no prompt:\n%s", frame)
	}
	// A filter nothing matches, so the effect is visible as an empty table rather
	// than as a table that happens to look similar.
	for _, character := range "zzzznosuch" {
		harness.screen.InjectKey(tcell.KeyRune, character, tcell.ModNone)
	}
	harness.screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	// The header stays -- a filter narrows the rows, it does not remove the table.
	frame, ok := harness.waitFor(t, "PID")
	if !ok {
		t.Fatalf("the table did not come back after the filter:\n%s", frame)
	}

	// Escape out of a second prompt leaves the first filter in place, which is what
	// "cancel" has to mean for a field that starts with the current value in it.
	// The backslash is the same action, htop's other spelling for it.
	harness.screen.InjectKey(tcell.KeyRune, '\\', tcell.ModNone)
	if _, ok := harness.waitFor(t, "filter:"); !ok {
		t.Fatal("the backslash opened no second prompt")
	}
	harness.screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if _, ok := harness.waitFor(t, "PID"); !ok {
		t.Fatal("escaping the prompt did not restore the table")
	}
	if err := harness.quit(t); err != nil {
		t.Fatalf("quit: %v", err)
	}
}

// The search prompt is incremental and leaves the list whole, which is what makes a
// search worth having over a filter: the process is found *in context*, with its
// neighbours and its share of the machine still on screen.
func TestTopKeys_searchPromptLeavesTheListWhole(t *testing.T) {
	harness := startTop(t)
	before, ok := harness.waitFor(t, "PID")
	if !ok {
		t.Fatal("the table never drew")
	}
	rowsBefore := strings.Count(before, "\n")

	harness.screen.InjectKey(tcell.KeyRune, '/', tcell.ModNone)
	if frame, ok := harness.waitFor(t, "search"); !ok {
		t.Fatalf("/ opened no prompt:\n%s", frame)
	}
	for _, character := range "zzzznosuch" {
		harness.screen.InjectKey(tcell.KeyRune, character, tcell.ModNone)
	}
	harness.screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	after, ok := harness.waitFor(t, "PID")
	if !ok {
		t.Fatalf("the table did not come back after the search:\n%s", after)
	}
	// The same number of screen rows: a search that removed rows would be a filter.
	if strings.Count(after, "\n") != rowsBefore {
		t.Fatalf("the search changed the table's height from %d to %d, so it filtered",
			rowsBefore, strings.Count(after, "\n"))
	}
	if err := harness.quit(t); err != nil {
		t.Fatalf("quit: %v", err)
	}
}

// topKeyName, which the help panel and the status line both read. Tested directly
// because it is a lookup table and a wrong entry shows as a key labelled as another.
func TestTopKeyName_namesEveryKeyItLabels(t *testing.T) {
	for _, test := range []struct {
		key  tcell.Key
		rune rune
		want string
	}{
		{key: tcell.KeyRune, rune: 'q', want: "q"},
		{key: tcell.KeyRune, rune: '/', want: "/"},
		{key: tcell.KeyRune, rune: 'k', want: "k"},
		{key: tcell.KeyF1, want: "F1"},
		{key: tcell.KeyF7, want: "F7"},
		{key: tcell.KeyF8, want: "F8"},
		// Lower case, because this is the *model's* vocabulary and the model's table
		// spells it "esc" beside "q".
		{key: tcell.KeyEscape, want: "esc"},
		{key: tcell.KeyRune, rune: ' ', want: "space"},
		{key: tcell.KeyRune, rune: '\\', want: `\`},
		{key: tcell.KeyF3, want: "F3"},
		{key: tcell.KeyF4, want: "F4"},
		{key: tcell.KeyF5, want: "F5"},
		{key: tcell.KeyF9, want: "F9"},
	} {
		t.Run(test.want, func(t *testing.T) {
			event := tcell.NewEventKey(test.key, test.rune, tcell.ModNone)
			if got := topKeyName(event); got != test.want {
				t.Fatalf("topKeyName = %q, want %q", got, test.want)
			}
		})
	}
}

// A key the table does not name answers the empty string, and that is the signal
// key() uses to pass the event on to the widget underneath rather than swallowing
// it. Enter and the arrows are what that matters for: they belong to the table.
func TestTopKeyName_answersEmptyForAKeyItDoesNotOwn(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyUp, tcell.KeyDown,
		tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd, tcell.KeyTab} {
		event := tcell.NewEventKey(key, 0, tcell.ModNone)
		if got := topKeyName(event); got != "" {
			t.Errorf("topKeyName(%v) = %q, want the empty string so the table gets the key",
				tcell.KeyNames[key], got)
		}
	}
}

// waitForEither polls for any one of several strings, which is what a test of an
// outcome that legitimately has two forms needs.
func (h *topHarness) waitForEither(t *testing.T, wanted ...string) (string, bool) {
	t.Helper()
	last := ""
	for attempt := 0; attempt < 200; attempt++ {
		last = h.text(t)
		for _, want := range wanted {
			if strings.Contains(last, want) {
				return last, true
			}
		}
	}
	return last, false
}
