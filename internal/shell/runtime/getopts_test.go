package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_getoptsParsesOptionArgumentAndIsDiscoverable_whenParsingPositionalParameters(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "set -- -b bee rest\ngetopts ab: opt\necho $opt-$OPTARG-$OPTIND\ncommand -v getopts\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "b-bee-3\ngetopts\n" {
		t.Fatalf("expected getopts output %q, got %q", "b-bee-3\ngetopts\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}
