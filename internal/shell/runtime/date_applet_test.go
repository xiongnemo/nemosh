package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_datePrintsFormattedUTCEpoch_whenRunWithDateAndFormat(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "date -u -d @0 +%Y-%m-%dT%H:%M:%SZ\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got, want := stdout.String(), "1970-01-01T00:00:00Z\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_datePrintsUnixSeconds_whenRunWithEpochAndSecondsFormat(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "date -u -d @1 +%s\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got, want := stdout.String(), "1\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
}

func TestRuntime_dateSucceedsWithNonEmptyOutput_whenRunWithoutOperands(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "date\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := strings.TrimSpace(stdout.String()); got == "" {
		t.Fatalf("expected non-empty stdout, got %q", stdout.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_dateRejectsSetTime_whenRunWithSetOption(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "date -s @0\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); !strings.Contains(got, "unsupported") {
		t.Fatalf("expected unsupported diagnostic, got %q", got)
	}
}
