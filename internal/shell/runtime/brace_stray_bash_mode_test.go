package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a `{` with no `}` to match it is a character, and brace expansion goes on to the
// next one, as bash's does: `{{a,b}` is {a and {b, `{a,{b,c}` is {a,b and {a,c. nemosh and sh
// leave such a word as written from the stray `{` on, which is what busybox, with no brace
// expansion at all, prints for every one of these. Measured 2026-11-22 with Git for Windows'
// bash 5.3; Oils' brace-expansion.test.sh, `{ in expansion`.
func TestBraceStray_bashModeGoesOnToTheNextBrace(t *testing.T) {
	script := "echo {{a,b} x{{a,b}y {a,{b,c} {a,b {a,b}}\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "{a {b x{ay x{by {a,b {a,c {a,b a} b}\n"},
		{bash: false, want: "{{a,b} x{{a,b}y {a,{b,c} {a,b a} b}\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if stdout.String() != test.want {
			t.Errorf("bash mode %v: stdout %q, want %q; stderr %q", test.bash, stdout.String(), test.want, stderr.String())
		}
	}
}
