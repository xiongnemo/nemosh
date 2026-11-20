package applets

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Files, as busybox-w32's vi.c has them -- its messages, 3132-3305 and 3249-3259 -- with vim's
// :N for the previous file, :r !COMMAND, and +COMMAND before the files.

// viSh is a ProgramRunner whose sh writes out and keeps what it was asked to run; nothing is
// launched.
type viSh struct {
	out  string
	runs *[]string
}

func (s viSh) Program(ctx context.Context, name string) (Applet, bool) {
	if name != "sh" {
		return nil, false
	}
	return simpleApplet{name: "sh", runContext: func(_ context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		*s.runs = append(*s.runs, strings.Join(args, " "))
		_, err := io.WriteString(stdout, s.out)
		return err
	}}, true
}

// newFilesVi opens files in a directory that holds them, with sh and an environment given.
func newFilesVi(t *testing.T, files map[string]string, environ []string, out string, operands ...string) (*viEditor, tcell.SimulationScreen, string, *[]string) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runs := &[]string{}
	ctx := WithProgramRunner(WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: environ}), viSh{out: out, runs: runs})
	editor := openVi(ctx, operands, false)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(60, 6)
	editor.screen, editor.width, editor.height = screen, 60, 6
	editor.draw()
	return editor, screen, dir, runs
}

func TestVi_editsFilesInTurn(t *testing.T) {
	files := map[string]string{"a.txt": "a\n", "b.txt": "b\n"}
	editor, screen, dir, _ := newFilesVi(t, files, nil, "", "a.txt", "b.txt")
	for _, step := range []struct{ keys, want string }{
		{":q\r", "1 more file(s) to edit"},
		{":N\r", "No previous files to edit"},
		{"x:n\r", "No write since last change (:n! overrides)"},
		{":n!\r", "'b.txt' 1L, 2C"},
		{":n\r", "No more files to edit"},
		{":rew\r", "'a.txt' 1L, 2C"},
		{":wn\r", "'b.txt' 1L, 2C"},
		{":prev\r", "'a.txt' 1L, 2C"},
		{"x:wq\r", "1 more file(s) to edit"},
		{":e b.txt\r", "'b.txt' 1L, 2C"},
		{":e #\r", "'a.txt' 1L, 1C"},
		{"ix\x1b:e\r", "No write since last change (:e! overrides)"},
		{":e!\r", "'a.txt' 1L, 1C"},
		{":e new.txt\r", "'new.txt' [New file] 1L, 1C"},
	} {
		typeVi(editor, step.keys)
		if got := viRow(t, screen, 5); got != step.want {
			t.Errorf("after %q said %q, want %q", step.keys, got, step.want)
		}
	}
	if got := readViFile(t, filepath.Join(dir, "a.txt")); got != "\n" {
		t.Errorf("a.txt holds %q: :wn wrote the x of a, and :wq the second x", got)
	}
	// :e leaves the place in the list of files, as vim's does, so one is still to come.
	typeVi(editor, ":q!\r")
	if !editor.quit {
		t.Error(":q! did not quit")
	}
}

func TestVi_readsFilesAndCommandsIn(t *testing.T) {
	files := map[string]string{"f.txt": "one\ntwo\n", "in.txt": "x\ny\n"}
	for _, test := range []struct{ name, keys, want, message string }{
		{"after the line", ":r in.txt\r", "one\nx\ny\ntwo", "'in.txt' 2L, 4C"},
		{"before the first", ":0r in.txt\r", "x\ny\none\ntwo", "'in.txt' 2L, 4C"},
		{"after the last", ":$r in.txt\r", "one\ntwo\nx\ny", "'in.txt' 2L, 4C"},
		{"a command's output, vim's", ":r !date\r", "one\nout1\nout2\ntwo", ""},
		{"a file not there", ":r nosuch\r", "one\ntwo", "'nosuch' No such file or directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor, screen, _, runs := newFilesVi(t, files, nil, "out1\nout2\n", "f.txt")
			typeVi(editor, test.keys)
			if got := viText(editor); got != test.want {
				t.Errorf("the buffer is %q, want %q", got, test.want)
			}
			if test.message != "" {
				if got := viRow(t, screen, 5); got != test.message {
					t.Errorf("said %q, want %q", got, test.message)
				}
			}
			if strings.Contains(test.keys, "!") && (len(*runs) != 1 || (*runs)[0] != "-c date") {
				t.Errorf("sh ran %q", *runs)
			}
		})
	}
}

// :!COMMAND runs sh -c with % and # for the file names; :f renames the buffer.
func TestVi_runsCommandsAndRenames(t *testing.T) {
	editor, screen, dir, runs := newFilesVi(t, map[string]string{"f.txt": "a\n", "g.txt": "b\n"}, nil, "", "f.txt")
	typeVi(editor, ":e g.txt\r:e #\r:!cat % \\% #\r")
	if len(*runs) != 1 || (*runs)[0] != "-c cat f.txt % g.txt" {
		t.Errorf("sh ran %q", *runs)
	}
	typeVi(editor, ":1!ls\r")
	if got := viRow(t, screen, 5); got != "Range not allowed" {
		t.Errorf(":1! said %q", got)
	}
	typeVi(editor, ":f h.txt\r")
	if got := viRow(t, screen, 5); got != "- h.txt 1/1 100%" {
		t.Errorf(":f said %q", got)
	}
	typeVi(editor, ":w\r")
	if got := readViFile(t, filepath.Join(dir, "h.txt")); got != "a\n" {
		t.Errorf("h.txt holds %q", got)
	}
}

// What runs before the first key: $EXINIT, or else ~/.exrc, then -c and vim's +COMMAND; -b keeps
// a file's carriage returns as they are.
func TestVi_startsWithItsCommands(t *testing.T) {
	text := map[string]string{"f.txt": "a\r\nfind me\r\n", ".exrc": "\" a comment\nset nu\n"}
	t.Run("EXINIT", func(t *testing.T) {
		editor, _, _, _ := newFilesVi(t, text, []string{"EXINIT=set ts=4"}, "", "f.txt")
		editor.start(nil, false)
		if editor.tabstop != 4 || editor.number {
			t.Errorf("tabstop %d, number %v", editor.tabstop, editor.number)
		}
	})
	t.Run(".exrc", func(t *testing.T) {
		editor, _, dir, _ := newFilesVi(t, text, nil, "", "f.txt")
		editor.view = hostProcessView{cwd: dir, environ: []string{"HOME=" + dir}}
		editor.start([]string{"/me", "set ic"}, false)
		if !editor.number || !editor.ignoreCase || editor.row != 1 || editor.col != 5 {
			t.Errorf("number %v, ignorecase %v, cursor %d,%d", editor.number, editor.ignoreCase, editor.row, editor.col)
		}
	})
	t.Run("-b", func(t *testing.T) {
		editor, _, dir, _ := newFilesVi(t, text, nil, "", "f.txt")
		editor.start(nil, true)
		typeVi(editor, ":wq\r")
		if got := viText(editor); got != "a\r\nfind me\r" {
			t.Errorf("the buffer is %q", got)
		}
		if got := readViFile(t, filepath.Join(dir, "f.txt")); got != "a\r\nfind me\r\n" {
			t.Errorf("f.txt holds %q", got)
		}
	})
}
