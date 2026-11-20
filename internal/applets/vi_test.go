package applets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// The status lines and messages these tests expect are busybox-w32's vi's, read off its screen
// by a console robot on 2026-11-21 (`busybox vi f.txt` over "one\ntwo\nthree\n"), and vi.c's
// format strings; none is what this vi happened to print.

// newTestVi opens name in dir, as `vi name` does, on a simulation screen of the given size.
func newTestVi(t *testing.T, dir, name string, readOnly bool, width, height int) (*viEditor, tcell.SimulationScreen) {
	t.Helper()
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	var operands []string
	if name != "" {
		operands = []string{name}
	}
	editor := openVi(ctx, operands, readOnly)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, height)
	editor.screen, editor.width, editor.height = screen, width, height
	editor.draw()
	return editor, screen
}

// viFixture writes text to f.txt in a new directory.
func viFixture(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// typeVi presses keys as a terminal sends them, a control character as itself, and draws.
func typeVi(e *viEditor, keys string) {
	for _, r := range keys {
		e.handle(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		e.draw()
	}
}

func viRow(t *testing.T, screen tcell.SimulationScreen, row int) string {
	t.Helper()
	width, _ := screen.Size()
	return rowOf(t, screen, row, width)
}

func readViFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestVi_drawsTheFileAndBusyboxsStatusLine(t *testing.T) {
	dir := viFixture(t, "one\ntwo\nthree\n")
	_, screen := newTestVi(t, dir, "f.txt", false, 40, 6)
	for row, want := range []string{"one", "two", "three", "~", "~", "- f.txt 1/3 33%"} {
		if got := viRow(t, screen, row); got != want {
			t.Errorf("row %d is %q, want %q", row, got, want)
		}
	}
	if x, y, visible := screen.GetCursor(); x != 0 || y != 0 || !visible {
		t.Errorf("the cursor is at %d,%d (shown %v), want 0,0", x, y, visible)
	}
}

func TestVi_writesWhatWasTypedAndQuits(t *testing.T) {
	dir := viFixture(t, "one\ntwo\nthree\n")
	editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)

	typeVi(editor, "ihello ")
	if got := viRow(t, screen, 5); got != "I f.txt [Modified] 1/3 33%" {
		t.Errorf("inserting, the status line is %q", got)
	}
	typeVi(editor, "\x1b:q\r")
	if got := viRow(t, screen, 5); got != "No write since last change (:q! overrides)" || editor.quit {
		t.Errorf(":q on a changed buffer: %q, quit %v", got, editor.quit)
	}
	if x, _, _ := screen.GetCursor(); x != 5 {
		t.Errorf("after Escape the cursor is at column %d, want 5, on the space", x)
	}
	typeVi(editor, ":w\r")
	if got := viRow(t, screen, 5); got != "'f.txt' 3L, 20C" {
		t.Errorf(":w said %q", got)
	}
	if got := readViFile(t, filepath.Join(dir, "f.txt")); got != "hello one\ntwo\nthree\n" {
		t.Errorf("the file holds %q", got)
	}
	typeVi(editor, ":q\r")
	if !editor.quit {
		t.Error(":q after :w did not quit")
	}
}

func TestVi_refusesWhatBusyboxRefuses(t *testing.T) {
	for _, test := range []struct {
		name, file, keys, want string
		readOnly               bool
	}{
		{name: "another file that is there", file: "f.txt", keys: ":w other.txt\r", want: "File exists (:w! overrides)"},
		{name: "a read-only buffer", file: "f.txt", readOnly: true, keys: ":w\r", want: "'f.txt' is read only"},
		{name: "no name", keys: ":w\r", want: "No current filename"},
		{name: "a key not yet here", file: "f.txt", keys: "K", want: "'K' is not implemented"},
		{name: "a control key", file: "f.txt", keys: "\x16", want: "'^V' is not implemented"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := viFixture(t, "one\ntwo\nthree\n")
			if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, test.file, test.readOnly, 60, 6)
			typeVi(editor, test.keys)
			if got := viRow(t, screen, 5); got != test.want {
				t.Errorf("said %q, want %q", got, test.want)
			}
			if got := readViFile(t, filepath.Join(dir, "other.txt")); got != "x\n" {
				t.Errorf("other.txt holds %q", got)
			}
		})
	}
}

// The status line of a buffer with no file, one that is not there yet, and -R, as busybox
// draws each before a key is pressed.
func TestVi_statusLineBeforeAKey(t *testing.T) {
	for _, test := range []struct {
		name, file, want string
		readOnly         bool
	}{
		{name: "no file", want: "- No file 1/1 100%"},
		{name: "a new file", file: "new.txt", want: "- new.txt 1/1 100%"},
		{name: "-R", file: "f.txt", readOnly: true, want: "- f.txt [Readonly] 1/3 33%"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, screen := newTestVi(t, viFixture(t, "one\ntwo\nthree\n"), test.file, test.readOnly, 40, 6)
			if got := viRow(t, screen, 5); got != test.want {
				t.Errorf("the status line is %q, want %q", got, test.want)
			}
		})
	}
}

// A new file is written with LF, as ed and nano here write one, where busybox-w32 writes CRLF;
// a file whose every line ends in CRLF goes back with CRLF, and a mixed one as it came.
func TestVi_keepsTheFilesLineEndings(t *testing.T) {
	for _, test := range []struct{ name, text, keys, want string }{
		{name: "a new file", keys: "inew\x1b:wq\r", want: "new\n"},
		{name: "CRLF", text: "a\r\nb\r\n", keys: "jA!\x1b:wq\r", want: "a\r\nb!\r\n"},
		{name: "mixed", text: "a\r\nb\n", keys: "jA!\x1b:wq\r", want: "a\r\nb!\n"},
		{name: "no final newline", text: "a", keys: "A!\x1b:wq\r", want: "a!\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.text != "" {
				if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			editor, _ := newTestVi(t, dir, "f.txt", false, 40, 6)
			typeVi(editor, test.keys)
			if got := readViFile(t, filepath.Join(dir, "f.txt")); got != test.want || !editor.quit {
				t.Errorf("the file holds %q, quit %v; want %q", got, editor.quit, test.want)
			}
		})
	}
}

// The loop itself: keys as tcell's terminal input delivers them on a Windows console, measured
// with a console robot writing key records (tmp/tcellprobe, 2026-11-21) -- Escape and a key
// typed within 50 ms of it are that key with Alt, so a quick `<Esc>:` is Alt+: -- and vi quits
// and gives the screen back. A cancelled context ends it too.
func TestVi_runsUntilItQuits(t *testing.T) {
	start := func(t *testing.T, ctx context.Context) (string, tcell.SimulationScreen, chan error) {
		dir := viFixture(t, "one\n")
		editor := openVi(WithProcessView(ctx, hostProcessView{cwd: dir}), []string{"f.txt"}, false)
		screen := tcell.NewSimulationScreen("UTF-8")
		// Waited for rather than the screen polled: the simulation screen's Init writes what
		// its GetContents reads, unguarded, and -race said so.
		ready := make(chan struct{})
		editor.ready = ready
		done := make(chan error, 1)
		go func() { done <- runVi(ctx, screen, editor) }()
		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			t.Fatal("vi drew nothing")
		}
		return filepath.Join(dir, "f.txt"), screen, done
	}
	wait := func(t *testing.T, done chan error) error {
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("vi did not return")
			return nil
		}
	}

	t.Run("quits", func(t *testing.T) {
		path, screen, done := start(t, context.Background())
		for _, r := range "ihi" {
			screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
		screen.InjectKey(tcell.KeyRune, ':', tcell.ModAlt)
		screen.InjectKey(tcell.KeyRune, 'x', tcell.ModNone)
		screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
		if err := wait(t, done); err != nil {
			t.Fatal(err)
		}
		if got := readViFile(t, path); got != "hione\n" {
			t.Errorf("the file holds %q", got)
		}
	})

	t.Run("a cancelled context ends it", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		_, _, done := start(t, ctx)
		cancel()
		if err := wait(t, done); err == nil {
			t.Error("vi ended without the context's error")
		}
	})
}

func TestVi_isAnAppletThatNeedsATerminal(t *testing.T) {
	stdout, _, status := runApplet(t, "vi", []string{"-H"}, "")
	if status != 0 || !strings.Contains(stdout, "busybox-w32's vi") {
		t.Errorf("vi -H: status %d, %q", status, stdout)
	}
	dir := viFixture(t, "one\n")
	_, stderr, status := runApplet(t, "vi", []string{filepath.Join(dir, "f.txt")}, "")
	if status == 0 || !strings.Contains(stderr, "not a terminal") {
		t.Errorf("vi with no terminal: status %d, stderr %q", status, stderr)
	}
}
