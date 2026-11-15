package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Leaving a modified buffer, as nano asks about it: Y writes it under the name offered and
// leaves, N leaves without writing, ^C goes back to it.

// writtenFile is a file holding "keep\n" in a directory of its own, which becomes the working
// directory, so the editor is opened as `nano name` is typed and offers that name back. A
// whole temporary path is wider than the line the prompt is drawn on.
func writtenFile(t *testing.T, name string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(name, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

// exited waits for the editor to leave.
func (h *editorHarness) exited(t *testing.T) {
	t.Helper()
	select {
	case <-h.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("the editor did not exit")
	}
}

// stillRunning fails if the editor has left.
func (h *editorHarness) stillRunning(t *testing.T) {
	t.Helper()
	select {
	case err := <-h.finished:
		t.Fatalf("the editor exited (%v)", err)
	case <-time.After(200 * time.Millisecond):
	}
}

// writeOut is nano's ^O and the Enter that accepts the name it offers.
func (h *editorHarness) writeOut(t *testing.T) {
	t.Helper()
	h.press(t, tcell.KeyCtrlO, 0)
	h.waitForScreen(t, "File Name to Write: ")
	h.press(t, tcell.KeyEnter, 0)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEditor_exitOnAModifiedBufferAsksToSaveIt(t *testing.T) {
	path := writtenFile(t, "guard.txt")
	harness := startEditor(t, "nano", path)
	harness.type_(t, "!")

	// When
	harness.press(t, tcell.KeyCtrlX, 0)
	harness.waitForScreen(t, "Save modified buffer?")
	harness.press(t, tcell.KeyRune, 'y')
	harness.waitForScreen(t, "File Name to Write: guard.txt")
	harness.press(t, tcell.KeyEnter, 0)

	// Then: written under the name offered, and gone
	harness.exited(t)
	if got := readFile(t, path); got != "!keep\n" {
		t.Fatalf("the file holds %q, want %q", got, "!keep\n")
	}
}

func TestEditor_exitWithoutSavingLeavesTheFileAlone(t *testing.T) {
	path := writtenFile(t, "guard.txt")
	harness := startEditor(t, "nano", path)
	harness.type_(t, "!")

	// When
	harness.press(t, tcell.KeyCtrlX, 0)
	harness.waitForScreen(t, "Save modified buffer?")
	harness.press(t, tcell.KeyRune, 'n')

	// Then
	harness.exited(t)
	if got := readFile(t, path); got != "keep\n" {
		t.Fatalf("the file changed without being saved: %q", got)
	}
}

// ^C goes back to the buffer, by either spelling: the Key a terminal sends and the rune with
// Ctrl the Windows console does. A key that answers nothing asks again rather than guessing.
func TestEditor_exitIsCancelledByControlC(t *testing.T) {
	for _, test := range []struct {
		name string
		key  tcell.Key
		ch   rune
		mods tcell.ModMask
	}{
		{name: "KeyCtrlC", key: tcell.KeyCtrlC},
		{name: "Ctrl and c, as Windows sends it", key: tcell.KeyRune, ch: 'c', mods: tcell.ModCtrl},
		{name: "Escape", key: tcell.KeyEscape},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writtenFile(t, "guard.txt")
			harness := startEditor(t, "nano", path)
			defer harness.stop(t)
			harness.type_(t, "!")
			harness.press(t, tcell.KeyCtrlX, 0)
			harness.waitForScreen(t, "Save modified buffer?")
			harness.press(t, tcell.KeyRune, 'q')
			harness.waitForScreen(t, "Save modified buffer?")

			// When
			harness.screen.InjectKey(test.key, test.ch, test.mods)

			// Then: still editing, still modified, nothing written
			harness.waitForScreen(t, "Cancelled")
			harness.stillRunning(t)
			harness.waitForScreen(t, "Modified")
			harness.type_(t, "?")
			if got := readFile(t, path); got != "keep\n" {
				t.Fatalf("cancelling wrote the file: %q", got)
			}
		})
	}
}

// ^C at the buffer says where the cursor is, as nano's does, and never leaves. tview stops an
// application on KeyCtrlC by itself, and tcell makes the Windows console's Ctrl and c that key,
// so ^C left the editor and dropped the buffer without a question.
func TestEditor_controlCAtTheBufferDoesNotLeave(t *testing.T) {
	for _, test := range []struct {
		name string
		key  tcell.Key
		ch   rune
		mods tcell.ModMask
	}{
		{name: "KeyCtrlC", key: tcell.KeyCtrlC},
		{name: "Ctrl and c, as Windows sends it", key: tcell.KeyRune, ch: 'c', mods: tcell.ModCtrl},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writtenFile(t, "guard.txt")
			harness := startEditor(t, "nano", path)
			defer harness.stop(t)
			harness.type_(t, "!")

			// When
			harness.screen.InjectKey(test.key, test.ch, test.mods)

			// Then
			harness.waitForScreen(t, "line 1/")
			harness.stillRunning(t)
			harness.waitForScreen(t, "Modified")
		})
	}
}

// ^O asks for the name, and Enter accepts it: the Enter of nano's ^O-Enter writes the file
// rather than going into the buffer as a new line.
func TestEditor_writeOutAsksForTheNameAndEnterWritesIt(t *testing.T) {
	path := writtenFile(t, "notes.txt")
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)
	harness.type_(t, "XY")

	// When
	harness.writeOut(t)

	// Then
	harness.waitForScreen(t, "Wrote")
	if got := readFile(t, path); got != "XYkeep\n" {
		t.Fatalf("the file holds %q, want %q", got, "XYkeep\n")
	}
}

// A name other than the file's writes there, which is how a copy is made, and the buffer is
// that file from then on.
func TestEditor_writeOutUnderAnotherName(t *testing.T) {
	path := writtenFile(t, "notes.txt")
	copied := "copy.txt"
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)
	harness.type_(t, "C")

	// When: the offered name rubbed out and another typed
	harness.press(t, tcell.KeyCtrlO, 0)
	harness.waitForScreen(t, "File Name to Write: notes.txt")
	for range len("notes.txt") {
		harness.press(t, tcell.KeyBackspace2, 0)
	}
	harness.type_(t, copied)
	harness.press(t, tcell.KeyEnter, 0)

	// Then
	harness.waitForScreen(t, "Wrote")
	if got := readFile(t, copied); got != "Ckeep\n" {
		t.Fatalf("the copy holds %q", got)
	}
	if got := readFile(t, path); got != "keep\n" {
		t.Fatalf("the original changed: %q", got)
	}
	harness.waitForScreen(t, "copy.txt")
}

// A write that fails does not leave: the buffer is all there is of the changes.
func TestEditor_exitStaysWhenTheWriteFails(t *testing.T) {
	path := writtenFile(t, "notes.txt")
	nowhere := filepath.Join("no-such-directory", "x.txt")
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)
	harness.type_(t, "!")

	// When
	harness.press(t, tcell.KeyCtrlX, 0)
	harness.waitForScreen(t, "Save modified buffer?")
	harness.press(t, tcell.KeyRune, 'y')
	harness.waitForScreen(t, "File Name to Write: notes.txt")
	for range len("notes.txt") {
		harness.press(t, tcell.KeyBackspace2, 0)
	}
	harness.type_(t, nowhere)
	harness.press(t, tcell.KeyEnter, 0)

	// Then
	harness.stillRunning(t)
	harness.waitForScreen(t, "Modified")
	if _, err := os.Stat(nowhere); err == nil {
		t.Fatal("a file was made where none could be")
	}
}

// micro asks its own question on ^Q, and y writes to the file it has and leaves.
func TestEditor_microAsksOnQuit(t *testing.T) {
	path := writtenFile(t, "guard.txt")
	harness := startEditor(t, "micro", path)
	harness.type_(t, "!")

	// When
	harness.press(t, tcell.KeyCtrlQ, 0)
	harness.waitForScreen(t, "Save changes to guard.txt before closing?")
	harness.press(t, tcell.KeyRune, 'y')

	// Then: written with no name asked for, as micro's ^S writes, and gone
	harness.exited(t)
	if got := readFile(t, path); !strings.HasPrefix(got, "!keep") {
		t.Fatalf("the file holds %q", got)
	}
}
