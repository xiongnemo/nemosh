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

func TestRuntime_chmodChangesFileModeAndIsDiscoverable_whenOctalModeOmitsWriteBits(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("sample"), 0o600); err != nil {
		t.Fatalf("expected chmod fixture write to succeed, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "chmod 444 "+filepath.ToSlash(path)+"\ncommand -v chmod\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "chmod\n" {
		t.Fatalf("expected chmod output %q, got %q", "chmod\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected chmod fixture stat to succeed, got %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("expected chmod to clear write bits, got %03o", info.Mode().Perm())
	}
}
