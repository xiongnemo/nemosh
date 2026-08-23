package applets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The editor, driven headlessly.
//
// tcell's simulation screen is what makes an interactive applet testable at all,
// and the harness is the one `top` already uses: the screen may only be read from
// tview's own goroutine, so every look goes through QueueUpdate. Reading it from
// the test goroutine races the drawing, which the race detector catches.
//
// These tests type into the editor and assert the file on disk, which is the only
// thing that ultimately matters about an editor.

type editorHarness struct {
	application *tview.Application
	screen      tcell.SimulationScreen
	session     *editorSession
	finished    chan error
}

func startEditor(t *testing.T, name, path string) *editorHarness {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	view := ProcessViewFromContext(context.Background())
	session, err := openEditorSession(WithProcessView(context.Background(), view), name, []string{path}, false)
	if err != nil {
		t.Fatalf("openEditorSession: %v", err)
	}
	harness := &editorHarness{screen: screen, session: session, finished: make(chan error, 1)}
	ready := make(chan *tview.Application, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	go func() {
		harness.finished <- runEditorApplication(ctx, session, editorKeyMapFor(name), screen, ready)
	}()
	select {
	case harness.application = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("the editor never started")
	}
	// One settled frame before anything is typed, so the first key is not lost to
	// a screen that has not been drawn yet.
	harness.text(t)
	return harness
}

func (h *editorHarness) text(t *testing.T) string {
	t.Helper()
	reply := make(chan string, 1)
	h.application.QueueUpdate(func() { reply <- simulationText(h.screen) })
	select {
	case text := <-reply:
		return text
	case <-time.After(10 * time.Second):
		t.Fatal("the editor stopped answering")
		return ""
	}
}

// press sends one key.
//
// It cannot wait for the effect: an injected screen event and a QueueUpdate
// callback go into the same select loop with no ordering between them, so a read
// taken straight afterwards may see the frame *before* the key was handled. That
// is what waitForScreen and waitForFile are for -- an asynchronous UI has to be
// asserted by polling the observable, not by assuming the next call is later.
func (h *editorHarness) press(t *testing.T, key tcell.Key, ch rune) {
	t.Helper()
	h.screen.InjectKey(key, ch, tcell.ModNone)
}

// waitForScreen polls until the screen holds the text, or fails.
func (h *editorHarness) waitForScreen(t *testing.T, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		last = h.text(t)
		if strings.Contains(last, want) {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the screen never showed %q; last frame was:\n%s", want, last)
	return ""
}

// waitForFile polls the file until it satisfies the predicate, which is how a
// save is confirmed without guessing when the key was handled.
func waitForFile(t *testing.T, path string, ready func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			last = string(data)
			if ready(last) {
				return last
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the file never reached the expected state; it holds %q", last)
	return ""
}

func (h *editorHarness) type_(t *testing.T, text string) {
	t.Helper()
	for _, character := range text {
		h.press(t, tcell.KeyRune, character)
	}
	// The typed text has to appear before anything is asserted about it.
	h.waitForScreen(t, text)
}

func (h *editorHarness) stop(t *testing.T) {
	t.Helper()
	h.application.Stop()
	select {
	case <-h.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("the editor did not stop")
	}
}

// The whole point of an editor: type something, write it out, and find it on
// disk.
func TestEditor_typesAndSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	// The file's contents are on screen, and so is its name.
	harness.waitForScreen(t, "first")
	harness.waitForScreen(t, "notes.txt")

	harness.type_(t, "XY")
	// Modified is shown, because an editor that loses work silently is the worst
	// kind and this is the one piece of state a user cannot otherwise see.
	harness.waitForScreen(t, "Modified")

	harness.press(t, tcell.KeyCtrlO, 0)
	harness.waitForScreen(t, "Wrote")
	waitForFile(t, path, func(text string) bool { return strings.Contains(text, "XY") })
	// And the modified flag clears once it is saved.
	if screen := harness.text(t); strings.Contains(screen, "Modified") {
		t.Fatalf("the title still says modified after saving: %q", screen)
	}
}

// The key map is chosen by the name, which is the whole reason there are two.
func TestEditor_eachNameHasItsOwnKeys(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		name     string
		save     tcell.Key
		wrong    tcell.Key
		inFooter string
	}{
		{name: "nano", save: tcell.KeyCtrlO, wrong: tcell.KeyCtrlS, inFooter: "^O Write Out"},
		{name: "micro", save: tcell.KeyCtrlS, wrong: tcell.KeyCtrlO, inFooter: "^S Save"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name+".txt")
			if err := os.WriteFile(path, []byte("body\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			harness := startEditor(t, test.name, path)
			defer harness.stop(t)

			// The footer advertises this name's keys.
			harness.waitForScreen(t, test.inFooter)
			harness.type_(t, "Z")
			// The other name's save key must not save here, or the two maps would
			// be the same map. Given a moment to be wrong in.
			harness.press(t, test.wrong, 0)
			time.Sleep(200 * time.Millisecond)
			if data, _ := os.ReadFile(path); strings.Contains(string(data), "Z") {
				t.Fatalf("%s saved on the other name's key", test.name)
			}
			harness.press(t, test.save, 0)
			waitForFile(t, path, func(text string) bool { return strings.Contains(text, "Z") })
		})
	}
}

// Quitting with unsaved changes must not lose them on one keystroke.
func TestEditor_guardsUnsavedChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guard.txt")
	if err := os.WriteFile(path, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := startEditor(t, "nano", path)
	harness.type_(t, "!")
	harness.press(t, tcell.KeyCtrlX, 0)
	harness.waitForScreen(t, "Unsaved")
	// Still running, so the buffer was not lost.
	select {
	case err := <-harness.finished:
		t.Fatalf("the editor exited on the first ^X, discarding the buffer (%v)", err)
	case <-time.After(200 * time.Millisecond):
	}
	// The second press leaves.
	harness.press(t, tcell.KeyCtrlX, 0)
	select {
	case <-harness.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("the editor did not exit on the second ^X")
	}
	// And the file on disk is untouched, because the changes were discarded
	// rather than written.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep\n" {
		t.Fatalf("the file changed without being saved: %q", data)
	}
}

func TestEditor_cutAndPasteALine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)

	harness.press(t, tcell.KeyCtrlK, 0)
	harness.waitForScreen(t, "Cut one line")
	harness.press(t, tcell.KeyCtrlO, 0)
	waitForFile(t, path, func(text string) bool { return !strings.Contains(text, "one") })

	harness.press(t, tcell.KeyCtrlU, 0)
	harness.waitForScreen(t, "Pasted one line")
	harness.press(t, tcell.KeyCtrlO, 0)
	waitForFile(t, path, func(text string) bool { return strings.Contains(text, "one") })
}

// -R opens read-only, and read-only has to stop editing while still allowing
// movement -- an editor that refused the arrow keys would be useless as a viewer.
func TestEditor_readOnlyRefusesEdits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := openEditorSession(context.Background(), "nano", []string{path}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.save("changed\n"); err == nil {
		t.Fatal("a read-only session saved")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Fatalf("the file changed under -R: %q", data)
	}
}

// A name that does not exist yet is how a new file is started, so it opens empty
// rather than failing.
func TestEditor_opensANewFile(t *testing.T) {
	dir := t.TempDir()
	session, err := openEditorSession(context.Background(), "nano", []string{filepath.Join(dir, "fresh.txt")}, false)
	if err != nil {
		t.Fatalf("opening a new file failed: %v", err)
	}
	if !session.isNew {
		t.Fatal("a missing file was not marked new")
	}
	if session.text != "" {
		t.Fatalf("a new buffer is not empty: %q", session.text)
	}
	if err := session.save("written\n"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "fresh.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "written\n" {
		t.Fatalf("the new file holds %q", data)
	}
}

// Bytes are written as they arrived: this editor does not decode, so it cannot
// re-encode, and a UTF-16 or Latin-1 file keeps its encoding. That is the same
// rule `sed -i` follows.
func TestEditor_savesBytesUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "utf16.txt")
	original := []byte{0xff, 0xfe, 'h', 0x00, 'i', 0x00}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	session, err := openEditorSession(context.Background(), "nano", []string{path}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.save(session.text); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("saving an untouched buffer changed the bytes: %v became %v", original, data)
	}
}

// -H lists the subset, in the manner of `busybox vi -H`. Every key it names must
// actually be bound, which is what stops the list drifting from the editor.
func TestEditor_featureListMatchesTheBindings(t *testing.T) {
	for _, name := range []string{"nano", "micro"} {
		keys := editorKeyMapFor(name)
		var listed strings.Builder
		if err := keys.writeFeatures(&listed); err != nil {
			t.Fatal(err)
		}
		text := listed.String()
		if !strings.Contains(text, name+" implements these features") {
			t.Fatalf("%s -H does not name itself: %q", name, text)
		}
		for _, binding := range keys.bindings {
			if !strings.Contains(text, binding.describes) {
				t.Fatalf("%s -H omits the bound key %q", name, binding.label)
			}
		}
		// And it says what is absent, because an editor that silently lacks
		// replace is worse than one that says it lacks it. Highlighting was on this
		// list until it was implemented; soft wrap replaced it, because highlighting
		// needs one screen row to be one buffer line.
		for _, absent := range []string{"No multiple buffers", "No replace", "No soft wrap"} {
			if !strings.Contains(text, absent) {
				t.Fatalf("%s -H does not admit %q", name, absent)
			}
		}
	}
}

// More than one file is refused rather than silently editing the first, since
// there are no multiple buffers to put the second in.
func TestEditor_refusesTwoFiles(t *testing.T) {
	applet := newNanoApplet()
	err := applet.Run(context.Background(), []string{"a.txt", "b.txt"}, strings.NewReader(""), &strings.Builder{}, &strings.Builder{})
	if err == nil {
		t.Fatal("nano accepted two files")
	}
	if !strings.Contains(err.Error(), "one file") {
		t.Fatalf("nano said %q, want it to name the limit", err)
	}
}
