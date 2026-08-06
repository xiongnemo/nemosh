package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// BusyBox applets that raise xfunc_error_retval still print through
// bb_perror_msg, so naming a status must not cost the diagnostic.
type statusMessageApplet struct{}

func (statusMessageApplet) Name() string { return "picky" }

func (statusMessageApplet) Run(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	return applets.ExitStatusMessage(2, errors.New("missing pattern"))
}

// A status with nothing to say stays silent; that is how `false` works.
type statusOnlyApplet struct{}

func (statusOnlyApplet) Name() string { return "quiet" }

func (statusOnlyApplet) Run(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	return applets.ExitStatus(3)
}

func TestRuntime_printsTheDiagnosticAndKeepsTheStatus_whenAnAppletNamesBoth(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	registry := applets.NewRegistry(statusMessageApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "picky\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if got, want := stderr.String(), "picky: missing pattern\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestRuntime_staysSilent_whenAnAppletNamesOnlyAStatus(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	registry := applets.NewRegistry(statusOnlyApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "quiet\n")

	// Then
	if status != 3 {
		t.Fatalf("expected status 3, got %d", status)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}
