package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An apostrophe that closes nothing in a double-quoted `${x-word}` is a character of the word,
// as busybox reads it: `"${X-it's}"` is it's. It was printed as written, `${X-it's}`, which
// neither reference does. After a pattern operator a single quote is one in both, and one that
// does not close is a syntax error: `"${foo%d'}"`, Oils' "Syntax error for single quote in
// double quote", which was printed as written too. A pair of them is read as bash reads it, as
// before. bash 5.3 refuses `"${X-it's}"` for the quote it does not close; both names read it
// as busybox, and bash 5.1, read it. Measured 2026-11-22 against busybox-w32 and Git for
// Windows' bash 5.3.
func TestParameters_readAnApostropheInADoubleQuotedWord(t *testing.T) {
	for _, bash := range []bool{false, true} {
		for _, test := range []struct {
			script, stdout, said string
		}{
			{script: "echo \"[${X-it's}]\"\necho \"[${X:-it's}]\"\necho \"[${X+it's}]\"\necho \"[${X-a'b'c}]\"\n", stdout: "[it's]\n[it's]\n[]\n[a'b'c]\n"},
			{script: "X=v\necho \"[${X:+don't}]\"\n", stdout: "[don't]\n"},
			{script: "foo=abcd\necho \"[${foo%'cd'}]\"\n", stdout: "[ab]\n"},
			{script: "foo=abcd\necho \"${foo%d'}\"\necho after\n", said: "unterminated quote"},
		} {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			if bash {
				rt.EnterBashMode()
			}
			status := rt.RunScript(context.Background(), test.script)
			wantStatus := 0
			if test.said != "" {
				wantStatus = 2
			}
			if status != wantStatus || stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
				t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want %d, %q and %q", bash, test.script, status, stdout.String(), stderr.String(), wantStatus, test.stdout, test.said)
			}
		}
	}
}
