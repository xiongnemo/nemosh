package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The dispatch is what running an applet costs the shell besides the applet's own work:
// from an argv already expanded to the applet returning. A shell runs it once per command,
// so whatever it allocates, a loop allocates that many times over.
func TestAllocations_dispatchingAnApplet(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { devNull.Close() })
	for _, test := range []struct {
		name   string
		stdout io.Writer
		staged bool
	}{
		{name: "at the top", stdout: &bytes.Buffer{}},
		{name: "to a file", stdout: devNull},
		{name: "in a pipeline stage", stdout: &bytes.Buffer{}, staged: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: a context that can be cancelled, as a script's and a session's can
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: test.stdout, Stderr: &stderr})
			if test.staged {
				var release func()
				ctx, rt, release = rt.ownStage(ctx, false)
				defer release()
			}

			// When: the argv made outside what is measured, as expansion makes it before
			// the dispatch
			args := []string{"true"}
			count := allocations(t, func() {
				if status := rt.runBuiltinOrProgram(ctx, args); status != 0 {
					t.Fatalf("true exited %d, stderr = %q", status, stderr.String())
				}
			})

			// Then
			if count != 0 {
				t.Fatalf("dispatching true took %d allocations; it takes none, see applet_frame.go", count)
			}
		})
	}
}
