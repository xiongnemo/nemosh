package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A text read in bash mode is read a command at a time, as bash reads it, so an alias defined,
// changed or removed is in force from the next command on: the rest of a text read whole is read
// again from there (script_reread.go), and not from inside the command that changed it -- `fi; x`
// is on that command's last line, and bash has read it already. Each answer is Git for Windows'
// bash 5.3's, measured 2026-11-22.
func TestAliases_bashModeReadsTheRestAgainWhenTheyChange(t *testing.T) {
	for _, test := range []struct{ name, script, stdout string }{
		{name: "not on the line of the command that defined it",
			script: "if true; then\n  alias x='echo hi'\nfi; x 2>/dev/null; echo st=$?\nx\n", stdout: "st=127\nhi\n"},
		{name: "unalias", script: "alias ll='echo ll'\nll a\nunalias ll\nll b 2>/dev/null; echo st=$?\n", stdout: "ll a\nst=127\n"},
		{name: "expand_aliases turned off", script: "alias ll='echo ll'\nll a\nshopt -u expand_aliases\nll b 2>/dev/null; echo st=$?\n", stdout: "ll a\nst=127\n"},
		{name: "defined again", script: "alias v='echo one'\nv\nalias v='echo two'\nv\n", stdout: "one\ntwo\n"},
		{name: "a function whose body opens on the next line", script: "alias ll='echo ll'\nf()\n{\n  ll\n}\nunalias ll\nf\n", stdout: "ll\n"},
		{name: "a group opened on a line of its own", script: "alias ll='echo ll'\n{\n  ll g\n}\n", stdout: "ll g\n"},
		{name: "a process substitution", script: "alias ll='echo ll'\nwhile read -r l; do echo got $l; done < <(ll p)\n", stdout: "got ll p\n"},
		{name: "after a here-document", script: "alias ll='echo ll'\ncat <<E\nbody\nE\nll h\n", stdout: "body\nll h\n"},
		{name: "before a syntax error", script: "alias LEFT='{'\nLEFT echo one; echo two; }\nif then\necho never\n", stdout: "one\ntwo\n"},
		{name: "a name the rest only quotes", script: "alias ll='echo ll'\necho 'll' \"ll\"\nll q\n", stdout: "ll ll\nll q\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			rt.EnterBashMode()
			rt.RunScript(context.Background(), "shopt -s expand_aliases\n"+test.script)
			if stdout.String() != test.stdout {
				t.Fatalf("stdout %q, stderr %q; want %q", stdout.String(), stderr.String(), test.stdout)
			}
		})
	}
}

// Whether the rest of a text reads differently is asked of the aliases that changed, as words a
// command's name could be: the rest is read again only when it could.
func TestAliases_readsDifferentlyOnlyWhereAChangedNameCouldBeRead(t *testing.T) {
	for _, test := range []struct {
		name     string
		old, now map[string]string
		lines    []string
		want     bool
	}{
		{name: "unused", now: map[string]string{"ll": "ls -l"}, lines: []string{"echo hello\n", "ls -l\n"}, want: false},
		{name: "a command's name", now: map[string]string{"ll": "ls -l"}, lines: []string{"echo a; ll\n"}, want: true},
		{name: "quoted or a part of a word", now: map[string]string{"ll": "ls -l"}, lines: []string{"echo 'll' -ll ll.txt\n"}, want: false},
		{name: "removed", old: map[string]string{"ll": "ls -l"}, lines: []string{"(ll)\n"}, want: true},
		{name: "in another's text", old: map[string]string{"s": "sudo "}, now: map[string]string{"s": "sudo ", "ll": "ls"}, lines: []string{"echo\n"}, want: false},
		{name: "brought in by another", old: map[string]string{"x": "ll "}, now: map[string]string{"x": "ll ", "ll": "ls"}, lines: []string{"echo\n"}, want: true},
		{name: "unchanged", old: map[string]string{"ll": "ls"}, now: map[string]string{"ll": "ls"}, lines: []string{"ll\n"}, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := readsDifferently(test.old, test.now, test.lines); got != test.want {
				t.Fatalf("readsDifferently = %v, want %v", got, test.want)
			}
		})
	}
}
