package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_expandsPositionalParameters_whenSetRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\necho $1-$2-$#\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one-two-2\n" {
		t.Fatalf("expected positional output %q, got %q", "one-two-2\n", got)
	}
}

func TestRuntime_shiftsPositionalParameters_whenShiftRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\nshift\necho $1-$#\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "two-1\n" {
		t.Fatalf("expected shifted output %q, got %q", "two-1\n", got)
	}
}

func TestRuntime_expandsAtAsFields_whenForLoopUsesAt(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\nfor item in $@\ndo\necho $item\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\n" {
		t.Fatalf("expected $@ loop output %q, got %q", "one\ntwo\n", got)
	}
}

func TestRuntime_returnsFailure_whenShiftRunsWithoutParameters(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "shift\n")

	// Then
	if status != 1 {
		t.Fatalf("expected empty shift status 1, got %d", status)
	}
}
