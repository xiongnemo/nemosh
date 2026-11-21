package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bind is bash's, which busybox has not got: what each form prints is bash 5.3's, from Oils'
// builtin-bind cases and from Git for Windows' bash driven in a console on 2026-11-21. A key
// bound to a function readline has and this line editor does not carry out is refused, where
// bash would make it work; one bound to no function at all binds nothing, status 0, as in bash.
func TestBind_asBashsBuiltin(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout, stderr string
	}{
		{name: "-l", script: "bind -l | grep -x accept-line\n", stdout: "accept-line\n"},
		{name: "-p and -P", script: "bind -p | grep vi-subst\nbind -P | grep vi-subst\nbind -p | grep -E 'yank$'\n",
			stdout: "# vi-subst (not bound)\nvi-subst is not bound to any keys\n\"\\C-y\": yank\n"},
		{name: "-s and -S", script: "bind -s\nbind -S\n"},
		{name: "-v and -V", script: "bind -v | grep blink-matching-paren\nbind -V | grep blink-matching-paren\n",
			stdout: "set blink-matching-paren off\nblink-matching-paren is set to `off'\n"},
		{name: "-q", script: "bind -q zz-bad; echo status=$?\nbind -q vi-subst; echo status=$?\nbind -q yank; echo status=$?\n",
			stdout: "status=1\nvi-subst is not bound to any keys.\nstatus=1\nyank can be invoked via \"\\C-y\".\nstatus=0\n",
			stderr: "bind: `zz-bad': unknown function name\n"},
		{name: "a key bound and taken away", script: "bind '\"\\C-o\\C-s\\C-h\": yank'\nbind -q yank\nbind -r '\\C-o\\C-s\\C-h'\nbind -q yank\n",
			stdout: "yank can be invoked via \"\\C-o\\C-s\\C-h\", \"\\C-y\".\nyank can be invoked via \"\\C-y\".\n"},
		{name: "a macro", script: "bind '\"\\C-y\": \"macro text\"'\nbind -s\nbind -q yank\n",
			stdout: "\"\\C-y\": \"macro text\"\nyank is not bound to any keys.\n"},
		{name: "-x and -X by keymap", script: "bind -m vi-insert -x '\"\\C-t\": echo vi'\nbind -m vi-insert -X\nbind -X\nbind -x '\"\\C-o\": \"echo quoted\"'\nbind -X\n",
			stdout: "\"\\C-t\" \"echo vi\"\n\"\\C-o\" \"echo quoted\"\n"},
		{name: "a variable", script: "bind 'set completion-ignore-case off'\nbind -v | grep completion-ignore-case\n",
			stdout: "set completion-ignore-case off\n"},
		{name: "a function this editor does not carry out", script: "bind '\"\\C-o\": vi-subst'; echo status=$?\nbind -p | grep -F '\"\\C-o\"'\n",
			stdout: "status=1\n", stderr: "bind: `vi-subst': a readline function this shell's line editor does not carry out\n"},
		{name: "no function at all", script: "bind '\"\\C-o\": no-such-fn'; echo status=$?\nbind -p | grep -F '\"\\C-o\"'\n", stdout: "status=0\n"},
		{name: "a keymap with no such name", script: "bind -m nosuch -p; echo status=$?\n", stdout: "status=1\n",
			stderr: "bind: `nosuch': invalid keymap name\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			rt.RunScript(context.Background(), test.script)
			if stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.stderr) || (test.stderr == "" && stderr.Len() != 0) {
				t.Fatalf("stdout %q, stderr %q; want %q and %q", stdout.String(), stderr.String(), test.stdout, test.stderr)
			}
		})
	}
}
