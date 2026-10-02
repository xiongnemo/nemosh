package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A word whose `[` no `]` closes is a literal, so it is looked up as the path it is rather than
// matched against every name in its directory, and it expands to what it did: itself. A `[` that
// is closed is still a bracket expression. `[` is what every `if [ ... ]` runs, and the
// directory was read for each.
func TestRuntime_anUnclosedBracketIsLookedUpNotMatched(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "[q"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + "'\necho [; echo x[; echo [q; echo [a]; echo *[; [ 1 -lt 2 ] && echo tested\n"
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)
	if want := "[\nx[\n[q\na\n*[\ntested\n"; stdout.String() != want || stderr.String() != "" {
		t.Errorf("got %q, %q; want %q", stdout.String(), stderr.String(), want)
	}
}
