package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// The editor's prompt machinery, and the three keys that use it: search, go to
// line, and help.
//
// All four were uncovered, which is a third of what each key map advertises. They
// are the hardest part of the editor to test and therefore the part most worth
// testing -- a prompt is a second input mode, so every key means something
// different while one is open, and getting that wrong makes the editor insert the
// search term into the file.

// A file with recognisable lines, so a jump or a match is visible on the screen
// without asserting cursor coordinates -- which the simulation screen does not
// expose and which are not the point anyway.
func writeNumberedFile(t *testing.T, name string, lines int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	var content strings.Builder
	for index := 1; index <= lines; index++ {
		content.WriteString("line ")
		content.WriteString(strings.Repeat("x", index%3))
		content.WriteString(" number ")
		content.WriteString(strings.TrimSpace(strings.Repeat(" ", 0)))
		content.WriteString(itoa(index))
		content.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// Search finds a line and says which, and the prompt's characters go into the
// prompt rather than into the file -- which is the thing that would be silently
// wrong if the prompt mode were not entered.
func TestEditor_searchFindsALineAndDoesNotTypeIntoTheFile(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 40)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	// ^W opens the prompt, which nano labels "Search: ".
	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")

	// Typing goes to the prompt. The label grows with each character, which is how
	// the test knows the keys are being collected rather than inserted.
	harness.type_(t, "number 37")
	harness.press(t, tcell.KeyEnter, 0)

	// The message names the line it found, one-based as a user counts.
	harness.waitForScreen(t, "Found on line 37")

	// And the file is untouched: not saved, and nothing typed into the buffer. The
	// buffer is checked through the title, which marks a modified file.
	frame := harness.text(t)
	if strings.Contains(frame, "modified") {
		t.Fatalf("searching marked the buffer modified:\n%s", frame)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatalf("the file changed during a search:\n%q", current)
	}
}

// A term that is not there says so rather than moving the cursor somewhere
// arbitrary or reporting a match it did not make.
func TestEditor_searchReportsWhatItCannotFind(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 10)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	harness.type_(t, "zzzznotthere")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "not found")
}

// An empty search is a no-op rather than a match on every line. Pressing ^W and
// Enter by accident should do nothing at all.
func TestEditor_emptySearchDoesNothing(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 10)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	harness.press(t, tcell.KeyEnter, 0)
	// Nothing is claimed to have been found. Polled through the title, which is
	// redrawn every frame, so a stale "Found" would still be visible.
	frame := harness.waitForScreen(t, "haystack.txt")
	if strings.Contains(frame, "Found on line") || strings.Contains(frame, "not found") {
		t.Fatalf("an empty search reported something:\n%s", frame)
	}
}

// Escape cancels a prompt, and afterwards the keys go back to editing. That
// second half is the one that matters: a prompt that does not close leaves every
// subsequent keystroke going somewhere invisible.
func TestEditor_escapeCancelsThePromptAndRestoresEditing(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 5)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	harness.type_(t, "abandoned")
	harness.press(t, tcell.KeyEscape, 0)
	harness.waitForScreen(t, "Cancelled")

	// Typing now reaches the buffer again.
	harness.type_(t, "TYPED")
	harness.press(t, tcell.KeyCtrlO, 0)
	saved := waitForFile(t, path, func(content string) bool { return strings.Contains(content, "TYPED") })
	// And the abandoned search term is not in the file.
	if strings.Contains(saved, "abandoned") {
		t.Fatalf("the cancelled search term reached the file:\n%q", saved)
	}
}

// Backspace inside a prompt edits the prompt, not the file. Without this the key
// would delete a character of the document behind the prompt.
func TestEditor_backspaceEditsThePromptNotTheFile(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 40)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	// "number 377" then one backspace leaves "number 37", which does match.
	harness.type_(t, "number 377")
	harness.press(t, tcell.KeyBackspace2, 0)
	harness.waitForScreen(t, "number 37")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Found on line 37")

	// Backspacing an empty prompt is not an error and does not reach the buffer.
	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	for index := 0; index < 3; index++ {
		harness.press(t, tcell.KeyBackspace2, 0)
	}
	harness.press(t, tcell.KeyEscape, 0)
	harness.waitForScreen(t, "Cancelled")
	frame := harness.text(t)
	if strings.Contains(frame, "modified") {
		t.Fatalf("backspacing an empty prompt modified the buffer:\n%s", frame)
	}
}

// Go to line, and the two answers that are not a line number.
func TestEditor_goesToALineAndRefusesWhatIsNotOne(t *testing.T) {
	path := writeNumberedFile(t, "long.txt", 60)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	// ^_ is nano's, and the prompt says what it wants.
	harness.press(t, tcell.KeyCtrlUnderscore, 0)
	harness.waitForScreen(t, "Line")
	harness.type_(t, "45")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Line 45")

	// A number past the end clamps to the last line rather than failing, which is
	// what nano does and is the more useful answer to "go to the end".
	harness.press(t, tcell.KeyCtrlUnderscore, 0)
	harness.waitForScreen(t, "Line")
	harness.type_(t, "9999")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Line 60")

	for _, answer := range []string{"abc", "0", "-4"} {
		harness.press(t, tcell.KeyCtrlUnderscore, 0)
		harness.waitForScreen(t, "Line")
		harness.type_(t, answer)
		harness.press(t, tcell.KeyEnter, 0)
		harness.waitForScreen(t, "not a line number")
	}
}

// Help lists the keys, and lists the ones this name actually binds -- which is
// what makes it useful rather than a second place for the footer to drift from.
func TestEditor_helpListsThisNamesKeys(t *testing.T) {
	for _, test := range []struct {
		name    string
		key     tcell.Key
		present string
		absent  string
	}{
		{name: "nano", key: tcell.KeyCtrlG, present: "^O Write Out", absent: "^S Save"},
		{name: "micro", key: tcell.KeyCtrlG, present: "^S Save", absent: "^O Write Out"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeNumberedFile(t, "help.txt", 3)
			harness := startEditor(t, test.name, path)
			defer harness.stop(t)

			harness.press(t, test.key, 0)
			// Waited on "Keys:" and not on the key label, because **the footer
			// already shows every label**. The first draft of this test waited for
			// the label, matched it in frame zero before ^G had been handled, and
			// then asserted against a frame that predated the help message -- the
			// same asynchronous-UI trap the harness comment warns about, in the
			// shape where the string being waited for was already true.
			frame := harness.waitForScreen(t, "Keys:")
			if !strings.Contains(frame, test.present) {
				t.Fatalf("%s's help does not list %q:\n%s", test.name, test.present, frame)
			}
			// The other map's key does appear nowhere -- neither in the help line
			// nor, since the footer is generated from the same table, in the footer.
			if strings.Contains(frame, test.absent) {
				t.Fatalf("%s's help mentions %q, which belongs to the other name:\n%s",
					test.name, test.absent, frame)
			}
		})
	}
}

// While a prompt is open, the editor's own action keys are collected as prompt
// input rather than firing. Otherwise typing a search term containing the letter
// bound to quit would quit.
func TestEditor_promptSwallowsTheActionKeys(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 10)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	// ^X is quit. With a prompt open it must not quit, and the editor must still
	// be answering afterwards -- which is what the next read proves.
	harness.press(t, tcell.KeyCtrlX, 0)
	harness.press(t, tcell.KeyEscape, 0)
	harness.waitForScreen(t, "Cancelled")
	// Still alive: a stopped application cannot answer a QueueUpdate.
	if frame := harness.text(t); !strings.Contains(frame, "haystack.txt") {
		t.Fatalf("the editor stopped or lost its title:\n%s", frame)
	}
}

// micro's own keys reach the same prompt, so the machinery is not nano-only.
func TestEditor_microUsesItsOwnSearchAndGoToKeys(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 40)
	harness := startEditor(t, "micro", path)
	defer harness.stop(t)

	// ^F rather than ^W.
	harness.press(t, tcell.KeyCtrlF, 0)
	harness.waitForScreen(t, "Search")
	harness.type_(t, "number 12")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Found on line 12")

	// ^L rather than ^_.
	harness.press(t, tcell.KeyCtrlL, 0)
	harness.waitForScreen(t, "Line")
	harness.type_(t, "7")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Line 7")
}

// Search wraps: a term earlier in the file than the cursor is still found, because
// the scan starts at the line after the cursor and comes round.
func TestEditor_searchWrapsPastTheEnd(t *testing.T) {
	path := writeNumberedFile(t, "haystack.txt", 30)
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	// Move well down the file first.
	harness.press(t, tcell.KeyCtrlUnderscore, 0)
	harness.waitForScreen(t, "Line")
	harness.type_(t, "25")
	harness.press(t, tcell.KeyEnter, 0)
	harness.waitForScreen(t, "Line 25")

	// Then search for something above it.
	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	harness.type_(t, "number 3\n"[:8])
	harness.press(t, tcell.KeyEnter, 0)
	// Line 3 is above the cursor, so finding it at all is the wrap.
	harness.waitForScreen(t, "Found on line 3")
}
