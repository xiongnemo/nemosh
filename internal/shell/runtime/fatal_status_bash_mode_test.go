package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A fatal expansion error -- `${x?message}`, an unset name under set -u -- ends a bash-mode
// shell with 1, and with 127 when it is a `-c` command string's own: bash 5.3 exits so from its
// command string, a function called there too, where a script, standard input, a subshell and
// a `$(...)` in the command string end with 1. Measured 2026-11-22 with Git for Windows' bash;
// busybox ends each with 2, which nemosh and sh keep.
func TestFatalStatus_bashModeCommandStringIs127(t *testing.T) {
	for _, test := range []struct {
		code, invocation, stdout string
		status                   int
	}{
		{code: "echo ${undef?message}; echo inside=$?", invocation: "c", status: 127},
		{code: "set -u; echo $undef; echo inside=$?", invocation: "c", status: 127},
		{code: "f() { echo ${undef?m}; }; f; echo after", invocation: "c", status: 127},
		{code: "(echo ${undef?m}); echo sub=$?", invocation: "c", stdout: "sub=1\n"},
		{code: "x=$(echo ${undef?m}); echo cs=$?", invocation: "c", stdout: "cs=1\n"},
		{code: "echo ${undef?message}; echo inside=$?", status: 1},
		{code: "echo ${undef?message}; echo inside=$?", invocation: "s", status: 1},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.SetInvocationMode(test.invocation)
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout {
			t.Errorf("%q as %q: status %d, stdout %q, stderr %q; want %d and %q", test.code, test.invocation, status, stdout.String(), stderr.String(), test.status, test.stdout)
		}
	}
	// As nemosh, busybox's 2.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.SetInvocationMode("c")
	if status := rt.RunScript(context.Background(), "echo ${undef?message}\n"); status != 2 {
		t.Errorf("as nemosh: status %d, stderr %q; want busybox's 2", status, stderr.String())
	}
}
