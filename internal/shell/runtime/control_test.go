package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_runsThenBranch_whenIfConditionSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "if true\nthen\necho yes\nelse\necho no\nfi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "yes\n" {
		t.Fatalf("expected then branch output %q, got %q", "yes\n", got)
	}
}

func TestRuntime_runsElseBranch_whenIfConditionFails(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "if false\nthen\necho yes\nelse\necho no\nfi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "no\n" {
		t.Fatalf("expected else branch output %q, got %q", "no\n", got)
	}
}

func TestRuntime_returnsBranchStatus_whenIfBranchCommandFails(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "if true\nthen\nfalse\nfi\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
}
