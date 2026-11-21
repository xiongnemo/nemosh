package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An inputrc is read as readline reads one: `set` lines, key bindings, `set keymap` choosing the
// keymap for the lines after it, `$if mode=...`, `$if term=...`, `$if Bash` and `$if version`
// with `$else` and `$endif`, nested, and `$include` of another file. bind -f reads a file so too.
func TestInputrc_isReadAsReadlineReadsOne(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other")
	text := "# a comment\n" +
		"set completion-ignore-case off\n" +
		"\"\\C-o\": beginning-of-line\n" +
		"$if mode=emacs\n\"\\C-x\\C-o\": \"in emacs\"\n$else\n\"\\C-x\\C-o\": \"in vi\"\n$endif\n" +
		"$if term=xterm\nset bell-style none\n$endif\n" +
		"$if Bash\n$if version >= 8.0\nset show-all-if-unmodified on\n$endif\n$endif\n" +
		"set keymap vi-insert\n\"\\C-t\": transpose-chars\n" +
		"$include " + filepath.ToSlash(other) + "\n"
	for name, body := range map[string]string{"inputrc": text, "other": "set mark-directories off\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.EnterBashMode()
	// term= is TERM's part before its first dash; pinned, since a runner's environment has one.
	rt.SetVariable("TERM", "xterm-256color")
	script := "bind -f '" + filepath.ToSlash(filepath.Join(dir, "inputrc")) + "'\n" +
		"bind -v | grep -E 'completion-ignore-case|bell-style|show-all-if-unmodified|mark-directories'\n" +
		"bind -p | grep -F '\"\\C-o\"'\nbind -s\nbind -m vi-insert -p | grep -F '\"\\C-t\"'\n"
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("exited %d", status)
	}
	want := "set completion-ignore-case off\nset mark-directories off\nset show-all-if-unmodified on\n" +
		"set bell-style none\n\"\\C-o\": beginning-of-line\n\"\\C-x\\C-o\": \"in emacs\"\n\"\\C-t\": transpose-chars\n"
	if stdout.String() != want {
		t.Fatalf("stdout\n%s\nwant\n%s", stdout.String(), want)
	}
}

// A session in bash mode reads $INPUTRC, or ~/.inputrc, before its startup files, as readline
// has read it by the time a bashrc's bind runs.
func TestInputrc_isTheSessionsInBashMode(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".inputrc"), []byte("set completion-ignore-case off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	rt.SetVariable("HOME", filepath.ToSlash(home))
	rt.ReadInputrc()
	if got := rt.ReadlineVariable("completion-ignore-case"); got != "off" {
		t.Errorf("after ~/.inputrc completion-ignore-case is %q", got)
	}
}
