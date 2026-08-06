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

// The runtime resolves device paths itself, so it reaches the host filesystem
// through its own OpenProcessInput rather than through the applets package's
// fallback. A fake ProcessView in an applet test therefore proves nothing about
// what the shell actually does; this drives the real view.
func TestRuntime_refuseToReadADirectoryThroughTheRuntimeProcessView(t *testing.T) {
	// Given
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "d"), 0o700); err != nil {
		t.Fatalf("expected the fixture directory to be created, got %v", err)
	}
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(tmp)+"'\ncat d\n")

	// Then
	if status != 1 {
		t.Fatalf("RunScript() = %d, want 1", status)
	}
	if got, want := stderr.String(), "cat: can't open 'd': Is a directory\n"; got != want {
		t.Fatalf("RunScript() wrote %q to stderr, want %q", got, want)
	}
}
