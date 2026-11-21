package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A conditional after `!` is still one: `! [[ -f $path || -p $path ]]`, as direnv's stdlib
// writes it, was split at its `||` -- the lexer took `[[` for a conditional only at the very
// start of a command -- and the whole stdlib was refused, `missing ]]`. And a `!` that is only
// an argument, `echo ! [[ x`, opened none. Each answer is Git for Windows' bash 5.3's, measured
// 2026-11-21; busybox has no [[.
func TestDoubleBracket_afterBangIsStillAConditional(t *testing.T) {
	for _, test := range []struct{ script, stdout string }{
		{script: "! [[ -f /nonexistent || -p /nonexistent ]]; echo $?\n", stdout: "0\n"},
		{script: "! [[ a == a && b == b ]]; echo $?\n", stdout: "1\n"},
		{script: "! [[ ab =~ ^(x|a)b$ ]]; echo $?\n", stdout: "1\n"},
		{script: "if ! [[ -f /nonexistent || -p /nonexistent ]]; then echo neither; fi\n", stdout: "neither\n"},
		{script: "[[ a ]] && ! [[ a == a && b == c ]] && echo after-and\n", stdout: "after-and\n"},
		{script: "echo ! [[ x\n", stdout: "! [[ x\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.RunScript(context.Background(), test.script)
		if stdout.String() != test.stdout || stderr.Len() != 0 {
			t.Errorf("%q: stdout %q, stderr %q; want %q", test.script, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
