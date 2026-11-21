package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A ${...} inside double quotes whose word holds a quoted newline goes on to the line after,
// as one inside no quotes did: busybox and bash both print one, a newline and two for
// a="${X-"one<newline>two"}", measured 2026-11-21. Only the unquoted one was read across the
// line; inside double quotes the inner quote was taken for the outer one's end, the line ended
// with it, and the script was "missing '}'". fzf's completion.bash sets a list of commands so,
// and the whole file was refused.
func TestBraceParameter_aQuotedNewlineInsideDoubleQuotes(t *testing.T) {
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	script := "a=\"${X-\"one\ntwo\"}\"\necho \"[$a]\"\nb=\"${X:-\"x\"}${X-\"y\nz\"}\"\necho \"[$b]\"\n"
	if status := rt.RunScript(context.Background(), script); status != 0 || stdout.String() != "[one\ntwo]\n[xy\nz]\n" {
		t.Fatalf("status %d, stdout %q", status, stdout.String())
	}
}
