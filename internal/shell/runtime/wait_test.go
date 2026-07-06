package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_waitSucceedsAndIsDiscoverable_whenNoJobsAreKnown(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "wait\ncommand -v wait\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "wait\n" {
		t.Fatalf("expected wait output %q, got %q", "wait\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}
