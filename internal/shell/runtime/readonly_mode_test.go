package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A write to a read-only variable ends a busybox script, status 2, wherever it is made. bash
// says `x: readonly variable` and goes on: an assignment statement, or an assignment inside
// $((...)), abandons the rest of its line, status 1; a prefix assignment is refused and the
// command runs; export, readonly, declare, local, unset, for, read and ((...)) answer 1 and the
// line goes on. Bash mode takes bash's, and nemosh and sh keep busybox's. Every answer was
// measured 2026-11-21, a script per case, with Git for Windows' bash 5.3 and busybox-w32.
func TestReadonly_bashModeRefusesAsBashDoes(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
	}{
		{code: "x=2; echo same=$?", stdout: "next=1\n", said: "x: readonly variable"},
		{code: "x+=2; echo same=$?", stdout: "next=1\n", said: "x: readonly variable"},
		{code: "x=(1 2); echo same=$?", stdout: "next=1\n", said: "x: readonly variable"},
		{code: "echo $((x=5)); echo same=$?", stdout: "next=1\n", said: "x: readonly variable"},
		{code: "x=2 true; echo same=$?", stdout: "same=0\nnext=0\n", said: "x: readonly variable"},
		{code: "x=2 :; echo same=$?", stdout: "same=0\nnext=0\n", said: "x: readonly variable"},
		{code: "export x=2; echo same=$?", stdout: "same=1\nnext=0\n", said: "line 2: x: readonly variable"},
		{code: "readonly x=2; echo same=$?", stdout: "same=1\nnext=0\n", said: "line 2: x: readonly variable"},
		{code: "declare x=2; echo same=$?", stdout: "same=1\nnext=0\n", said: "declare: x: readonly variable"},
		{code: "f() { local x=3; echo in-f=$?; }; f; echo same=$?", stdout: "in-f=1\nsame=0\nnext=0\n", said: "local: x: readonly variable"},
		{code: "unset x; echo same=$?", stdout: "same=1\nnext=0\n", said: "unset: x: cannot unset: readonly variable"},
		{code: "for x in 1; do echo body; done; echo same=$?", stdout: "same=1\nnext=0\n", said: "x: readonly variable"},
		{code: "read x <<< 3; echo same=$?", stdout: "same=1\nnext=0\n", said: "x: readonly variable"},
		{code: "((x=5)); echo same=$?", stdout: "same=1\nnext=0\n", said: "x: readonly variable"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), "readonly x=1\n"+test.code+"\necho next=$?\n")
		if status != 0 || stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.said) || strings.Contains(stderr.String(), "is read only") {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want 0, %q and %q", test.code, status, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
	// Under `set -o posix` bash's own POSIX mode ends the script, status 1, as Oils' bugs.test.sh
	// records it.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.EnterBashMode()
	if status := rt.RunScript(context.Background(), "set -o posix\nreadonly x=1\nx=2\necho hi\n"); status != 1 || stdout.Len() != 0 {
		t.Errorf("under set -o posix: status %d, stdout %q, stderr %q; want the end, 1", status, stdout.String(), stderr.String())
	}
	// As nemosh, busybox's: the script ends there, status 2.
	for _, code := range []string{"x=2", "x=2 true", "export x=2", "unset x", "for x in 1; do :; done"} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), "readonly x=1\n"+code+"\necho next=$?\n")
		if status != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "x: is read only") {
			t.Errorf("as nemosh %q: status %d, stdout %q, stderr %q; want busybox's end, 2", code, status, stdout.String(), stderr.String())
		}
	}
}
