package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A special builtin's error ends a busybox script: `.` of a file that is not there, a syntax
// error in eval's text, an option set has not got, local outside a function; and return
// outside one ends the script as exit does. bash, outside POSIX mode, answers each with a
// status and goes on with the line, and says break outside a loop, which busybox passes over in
// silence. Bash mode takes bash's, and nemosh and sh keep busybox's. Measured 2026-11-21, a
// script per case, with Git for Windows' bash 5.3 and busybox-w32.
func TestBuiltinError_bashModeGoesOnAsBashDoes(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
	}{
		{code: ". ./no-such-file", stdout: "same=1\nnext=0\n", said: "line 1: ./no-such-file: No such file or directory"},
		{code: "eval 'if'", stdout: "same=2\nnext=0\n"},
		{code: "set -Z", stdout: "same=2\nnext=0\n", said: "line 1: set: -Z: invalid option\nset: usage: set [-abefhkmnptuvxBCEHPT] [-o option-name] [--] [-] [arg ...]\n"},
		{code: "local v=1", stdout: "same=1\nnext=0\n", said: "line 1: local: can only be used in a function"},
		{code: "return 3", stdout: "same=2\nnext=0\n", said: "line 1: return: can only `return' from a function or sourced script"},
		{code: "break", stdout: "same=0\nnext=0\n", said: "line 1: break: only meaningful in a `for', `while', or `until' loop"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"; echo same=$?\necho next=$?\n")
		if status != 0 || stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.said) {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want 0, %q and %q", test.code, status, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
	// As nemosh, busybox's: the script ends there, and return ends it as exit does.
	for code, want := range map[string]int{". ./no-such-file": 2, "set -Z": 2, "local v=1": 2, "eval 'if'": 2, "return 3": 3} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if status := rt.RunScript(context.Background(), code+"; echo same=$?\necho next=$?\n"); status != want || stdout.Len() != 0 {
			t.Errorf("as nemosh %q: status %d, stdout %q; want busybox's end, %d", code, status, stdout.String(), want)
		}
	}
}
