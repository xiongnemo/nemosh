package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_runsExitTrap_whenScriptCompletes(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo bye' EXIT\necho hi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\nbye\n" {
		t.Fatalf("expected trap output %q, got %q", "hi\nbye\n", got)
	}
}

func TestRuntime_runsExitTrap_whenExitBuiltinStopsScript(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo bye' EXIT\nexit 7\necho unreachable\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
	if got := stdout.String(); got != "bye\n" {
		t.Fatalf("expected trap output %q, got %q", "bye\n", got)
	}
}
