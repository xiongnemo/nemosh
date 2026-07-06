package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_execSucceeds_whenNoOperands(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "exec\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}

func TestRuntime_execRunsCommandAndStopsScript_whenOperandSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "exec echo hi\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected exec output %q, got %q", "hi\n", got)
	}
}

func TestRuntime_execReturnsCommandStatusAndStopsScript_whenOperandFails(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "exec false\necho after\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}
