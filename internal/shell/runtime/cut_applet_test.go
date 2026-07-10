package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_cutCutsInputAndIsDiscoverable_whenRunFromScript(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: bytes.NewBufferString("abcdef\n"), Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cut -c 2-4\ncommand -v cut\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got, want := stdout.String(), "bcd\ncut\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_cutReturnsStatusTwo_whenRunWithInvalidOption(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cut -x\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "cut: invalid option -- x\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}
