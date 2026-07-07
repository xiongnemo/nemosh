package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_returnsLastPipelineStatusByDefault_whenEarlierCommandFails(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false | true\n")

	// Then
	if status != 0 {
		t.Fatalf("expected default pipeline status 0, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestRuntime_returnsLastNonzeroPipelineStatus_whenPipefailIsSet(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -o pipefail\nfalse | true\n")

	// Then
	if status != 1 {
		t.Fatalf("expected pipefail pipeline status 1, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestRuntime_keepsLastPipelineStatus_whenPipefailPipelineEndsWithFailure(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -o pipefail\necho hi | false\n")

	// Then
	if status != 1 {
		t.Fatalf("expected final failing pipeline status 1, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}
