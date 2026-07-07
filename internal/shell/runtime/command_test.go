package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_commandRunsCommand_whenOperandIsEcho(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "command echo hi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected command output %q, got %q", "hi\n", got)
	}
}

func TestRuntime_commandSucceeds_whenNoOperands(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "command\n")

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

func TestRuntime_commandVPrintsName_whenCommandIsKnown(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "echo"},
		{name: "cd"},
		{name: "readlink"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

			// When
			status := rt.RunScript(context.Background(), "command -v "+tt.name+"\n")

			// Then
			if status != 0 {
				t.Fatalf("expected status 0, got %d", status)
			}
			if got := stdout.String(); got != tt.name+"\n" {
				t.Fatalf("expected command -v output %q, got %q", tt.name+"\n", got)
			}
		})
	}
}

func TestRuntime_commandVReturnsOne_whenCommandIsMissing(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "command -v missing-name\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}
}
