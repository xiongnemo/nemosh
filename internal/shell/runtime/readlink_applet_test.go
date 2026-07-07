package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_readlinkFailsSilently_whenOperandIsRegularFile(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "regular.txt")
	if err := os.WriteFile(path, []byte("not-a-link"), 0o600); err != nil {
		t.Fatalf("expected readlink fixture write to succeed, got %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "readlink "+filepath.ToSlash(path)+"\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}
