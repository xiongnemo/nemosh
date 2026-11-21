package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Where bash's builtins and busybox's say a failure differently, bash mode says it as bash does,
// with bash's status: cd's, type's and command's, kill's, the numeric operand of exit, return,
// shift and break, alias's and getopts's. Measured 2026-11-21, a script per case, with Git for
// Windows' bash 5.3 and busybox-w32; nemosh and sh keep busybox's.
func TestBuiltinMessages_bashModeSaysThemAsBash(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
	}{
		{code: "cd /no/such/dir", stdout: "next=1\n", said: "line 1: cd: /no/such/dir: No such file or directory\n"},
		{code: "cd a b", stdout: "next=2\n", said: "line 1: cd: too many arguments\n"},
		{code: "type nosuchcmd", stdout: "next=1\n", said: "line 1: type: nosuchcmd: not found\n"},
		{code: "command -V nosuchcmd", stdout: "next=1\n", said: "line 1: command: nosuchcmd: not found\n"},
		{code: "command -v nosuchcmd", stdout: "next=1\n"},
		{code: "kill", stdout: "next=2\n", said: "kill: usage: kill [-s sigspec | -n signum | -sigspec] pid | jobspec ... or kill -l [sigspec]\n"},
		{code: "kill -9 nosuchjob", stdout: "next=1\n", said: "line 1: kill: `nosuchjob': not a pid or valid job spec\n"},
		{code: "exit x", stdout: "next=2\n", said: "line 1: exit: x: numeric argument required\n"},
		{code: "f() { return x; echo in-f; }; f", stdout: "next=2\n", said: "line 1: return: x: numeric argument required\n"},
		{code: "shift x", stdout: "next=2\n", said: "line 1: shift: x: numeric argument required\n"},
		{code: "for i in 1 2; do break x; echo in-loop; done; echo same", stdout: "next=1\n", said: "line 1: break: x: numeric argument required\n"},
		{code: "alias nosuch", stdout: "next=1\n", said: "line 1: alias: nosuch: not found\n"},
		{code: "unalias nosuch", stdout: "next=1\n", said: "line 1: unalias: nosuch: not found\n"},
		{code: "getopts", stdout: "next=2\n", said: "getopts: usage: getopts optstring name [arg ...]\n"},
		{code: "trap foo", stdout: "next=2\n", said: "trap: usage: trap [-Plp] [[action] signal_spec ...]\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"\necho next=$?\n")
		if status != 0 || stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want 0, %q and %q", test.code, status, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
	// As nemosh, busybox's, a few of them.
	for code, want := range map[string]string{
		"cd /no/such/dir": "next=2\n", "type nosuchcmd": "nosuchcmd: not found\nnext=127\n",
		"command -v nosuchcmd": "next=127\n", "alias nosuch": "next=1\n", "trap foo": "next=1\n",
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.RunScript(context.Background(), code+"\necho next=$?\n")
		if stdout.String() != want || strings.Contains(stderr.String(), ": not found") {
			t.Errorf("as nemosh %q: stdout %q, stderr %q; want busybox's %q", code, stdout.String(), stderr.String(), want)
		}
	}
}
