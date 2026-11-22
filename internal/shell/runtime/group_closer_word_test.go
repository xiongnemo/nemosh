package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A subshell's `)` ends the word after it, as an operator does. `(cmd)2>/dev/null` sends cmd's
// standard error away and keeps its output, and `(cmd)x` is a syntax error; the `2` joined the
// group and the stdout went instead, and the `x` was dropped and the group run. busybox-w32 and
// bash agree on all of these; measured 2026-11-22.
func TestGroupCloser_endsTheWordAfterIt(t *testing.T) {
	for _, test := range []struct {
		code, stdout string
		status       int
	}{
		{code: "(echo hi; echo e >&2)2>/dev/null", stdout: "hi\n"},
		{code: "x=$( (echo hi; echo e >&2)2>/dev/null ); echo \"[$x]\"", stdout: "[hi]\n"},
		{code: "(echo a >&2)2>&1 >/dev/null", stdout: "a\n"},
		{code: "(echo hi)x; echo st=$?", status: 2},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout || test.status == 0 && stderr.Len() > 0 {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want %d and %q", test.code, status, stdout.String(), stderr.String(), test.status, test.stdout)
		}
	}
}
