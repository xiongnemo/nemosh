package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Under `2>&1` an external command's stdout and stderr are one open file, and what it writes
// to the two reaches that file in the order it wrote it, as in both references. It was given
// two pipes, each copied by its own goroutine, so a file or a pipe got the lines in whatever
// order the copiers ran: every stdout line first, often, and the errors after.
func TestExternal_twoToOneKeepsTheOrder(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built, so there is no external command to run")
	}
	// Given: a child that alternates the two streams, line by line.
	child := "'" + filepath.ToSlash(binary) + "' -c 'for i in 1 2 3 4 5 6; do echo out$i; echo err$i >&2; done'"
	file := filepath.ToSlash(filepath.Join(t.TempDir(), "log"))
	want := "out1\nerr1\nout2\nerr2\nout3\nerr3\nout4\nerr4\nout5\nerr5\nout6\nerr6\n"
	for name, script := range map[string]string{
		"to a file":   child + " > '" + file + "' 2>&1; cat '" + file + "'\n",
		"to a pipe":   child + " 2>&1 | cat\n",
		"both, dup'd": "{ " + child + "; } 2>&1 | cat\n",
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), script)
			rt.CloseBatch(status)

			// Then
			if stdout.String() != want || status != 0 {
				t.Fatalf("got %q/%d, want %q/0; stderr %q", stdout.String(), status, want, stderr.String())
			}
		})
	}
}
