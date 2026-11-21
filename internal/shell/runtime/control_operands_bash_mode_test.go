package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Bash mode reads the operands of break, continue, exit, return and shift as bash 5.3 does:
// a `--` before them is passed over; break's and continue's count that is no number ends the
// script, status 2, and one of 0 or less is out of range, status 1, every loop left; a second
// operand to any of them is `too many arguments`, the rest of the line abandoned, status 2,
// whatever `set -e` says; `shift -1` is out of range, status 1. And break and continue in a
// function do not reach the caller's loops, which bash keeps apart. busybox reads each another
// way, and nemosh and sh keep busybox's. Measured 2026-11-22, a script per case, with Git for
// Windows' bash 5.3 and busybox-w32.
func TestControlOperands_bashModeReadsThemAsBash(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
		status             int
	}{
		{code: "for i in 1 2; do break x; echo in-loop; done; echo same\necho next", said: "line 1: break: x: numeric argument required\n", status: 2},
		{code: "for i in 1 2; do\n continue x\n echo in-loop\ndone\necho next", said: "line 2: continue: x: numeric argument required\n", status: 2},
		{code: "for i in 1 2; do break 99999999999999999999; echo in; done\necho next", said: "break: 99999999999999999999: numeric argument required\n", status: 2},
		{code: "for i in 1 2; do\n break 1 2\n echo in\ndone\necho next=$?", stdout: "next=2\n", said: "line 2: break: too many arguments\n"},
		{code: "for i in 1 2; do break 1 2; echo in; done; echo same=$?\necho next=$?", stdout: "next=2\n", said: "line 1: break: too many arguments\n"},
		{code: "for i in foo; do\n  continue 1 extra\ndone\necho status=$?", stdout: "status=2\n", said: "continue: too many arguments\n"},
		{code: "for i in 1 2; do for j in a b; do break 0; echo in; done; echo outer; done; echo same=$?", stdout: "same=1\n", said: "break: 0: loop count out of range\n"},
		{code: "for i in 1 2; do echo i=$i; continue -1; echo in; done; echo same=$?", stdout: "i=1\nsame=1\n", said: "continue: -1: loop count out of range\n"},
		{code: "for i in 1 2; do break -- 1; echo in; done; echo same=$?", stdout: "same=0\n"},
		{code: "for i in 1 2; do break ' 1 '; echo in; done; echo same=$?", stdout: "same=0\n"},
		{code: "break x; echo same=$?", stdout: "same=0\n", said: "break: only meaningful in a `for', `while', or `until' loop\n"},
		{code: "f() { continue; }\nfor i in 1 2; do f; echo in=$i; done; echo same=$?", stdout: "in=1\nin=2\nsame=0\n", said: "continue: only meaningful in a `for', `while', or `until' loop\n"},
		{code: "(exit -- 3); echo st=$?", stdout: "st=3\n"},
		{code: "f() { return -- 4; }; f; echo st=$?", stdout: "st=4\n"},
		{code: "exit 1 2; echo same=$?\necho next=$?", stdout: "next=2\n", said: "line 1: exit: too many arguments\n"},
		{code: "f() { return 1 2; echo in-f; }\ng() { f; echo in-g=$?; }\ng; echo same=$?\necho next=$?", stdout: "next=2\n", said: "return: too many arguments\n"},
		{code: "set -- a b c\nshift 1 extra; echo same=$?\necho next=$? $#", stdout: "next=2 3\n", said: "line 2: shift: too many arguments\n"},
		{code: "set -e; set -- a; shift 1 2; echo same=$?\necho next=$?", stdout: "next=2\n", said: "shift: too many arguments\n"},
		{code: "set -- a b; shift -1; echo same=$? $#", stdout: "same=1 2\n", said: "shift: -1: shift count out of range\n"},
		{code: "set -- a b c; shift -- 2; echo same=$? $#", stdout: "same=0 1\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"\n")
		said := stderr.String()
		if status != test.status || stdout.String() != test.stdout || !strings.HasSuffix(said, test.said) || strings.Count(said, "\n") != strings.Count(test.said, "\n") && !strings.Contains(test.code, "f() { continue; }") {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want %d, %q and %q", test.code, status, stdout.String(), said, test.status, test.stdout, test.said)
		}
	}
	// From a `-c` command string, outside a subshell, a second operand ends the shell, status 1,
	// as bash's parse_and_execute ends the string it discards from; in a subshell the line is
	// abandoned, as in a script.
	for _, test := range []struct {
		code, stdout string
		status       int
	}{
		{code: "for i in foo; do\n  continue 1 extra\ndone\necho status=$?", status: 1},
		{code: "set -- a b c\nshift 1 extra; echo same=$?\necho next=$?", status: 1},
		{code: "f() { return 1 2; }; f\necho next=$?", status: 1},
		{code: "(shift 1 2; echo same)\necho next=$?", stdout: "next=2\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.SetInvocationMode("c")
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout || !strings.Contains(stderr.String(), ": too many arguments\n") {
			t.Errorf("-c %q: status %d, stdout %q, stderr %q; want %d and %q", test.code, status, stdout.String(), stderr.String(), test.status, test.stdout)
		}
	}
	// As nemosh, busybox's: a count that is no number ends the script, and continue in a function
	// continues the caller's loop.
	for code, want := range map[string]string{
		"for i in 1 2; do break x; echo in; done\necho next":                    "",
		"f() { continue; }\nfor i in 1 2; do f; echo in=$i; done; echo same=$?": "same=0\n",
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.RunScript(context.Background(), code+"\n")
		if stdout.String() != want {
			t.Errorf("as nemosh %q: stdout %q, stderr %q; want busybox's %q", code, stdout.String(), stderr.String(), want)
		}
	}
}
