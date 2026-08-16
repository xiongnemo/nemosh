//go:build !windows

package runtime_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Nothing is invented here. A Unix login sets HOME before any shell starts, so a
// shell that finds it missing is running somewhere deliberately bare -- a
// container, a cron job -- and making one up would hide that rather than help.
func TestRuntime_inventsNoHomeOffWindows(t *testing.T) {
	// Given
	var stdout strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(nil)})
	if err != nil {
		t.Fatal(err)
	}

	// When
	rt.RunScript(t.Context(), "echo [$HOME]\n")

	// Then
	if got := strings.TrimSpace(stdout.String()); got != "[]" {
		t.Fatalf("HOME = %q, want it left unset", got)
	}
}
