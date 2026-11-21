package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Bash mode's wait refuses what it cannot take as bash's does: an operand that is no number and
// no job spec is said to be not a pid or valid job spec, status 1, where busybox's is `Illegal
// number`, 2; a pid that is no child is said, `pid N is not a child of this shell`, and is 127
// where it comes, where busybox says nothing and leaves the status as it was; and an option
// wait has not got, or -p without its name, is said with wait's usage, 2. -f, which waits for a
// job to end where there is job control, is taken under every name, being bash's alone: there is
// none here, and wait waits for the end already. Measured 2026-11-22, a script per case, with Git
// for Windows' bash 5.3 and busybox-w32.
func TestWait_bashModeRefusesAsBash(t *testing.T) {
	const usage = "wait: usage: wait [-fn] [-p var] [id ...]\n"
	for _, test := range []struct {
		code, stdout, said string
	}{
		{code: "wait zzz; echo st=$?", stdout: "st=1\n", said: "line 1: wait: `zzz': not a pid or valid job spec\n"},
		{code: "wait ''; echo st=$?", stdout: "st=1\n", said: "wait: `': not a pid or valid job spec\n"},
		{code: "wait 99999; echo st=$?", stdout: "st=127\n", said: "line 1: wait: pid 99999 is not a child of this shell\n"},
		{code: "(exit 3) & wait $! 99999; echo st=$?", stdout: "st=127\n", said: "wait: pid 99999 is not a child of this shell\n"},
		{code: "wait -Y; echo st=$?", stdout: "st=2\n", said: "line 1: wait: -Y: invalid option\n" + usage},
		{code: "wait -p; echo st=$?", stdout: "st=2\n", said: "line 1: wait: -p: option requires an argument\n" + usage},
		{code: "(exit 5) & wait -f $!; echo st=$?", stdout: "st=5\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.RunScript(context.Background(), test.code+"\n")
		if stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("%s: stdout %q, stderr %q; want %q and %q", test.code, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
	// As nemosh, busybox's, -f aside.
	for code, want := range map[string]string{
		"wait zzz; echo st=$?":                 "st=2\n",
		"(exit 3) & wait $! 99999; echo st=$?": "st=3\n",
		"(exit 5) & wait -f $!; echo st=$?":    "st=5\n",
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.RunScript(context.Background(), code+"\n")
		if stdout.String() != want || strings.Contains(stderr.String(), "not a child") {
			t.Errorf("as nemosh %q: stdout %q, stderr %q; want busybox's %q", code, stdout.String(), stderr.String(), want)
		}
	}
}
