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

// chmod filters a MODE with no class letters through the shell's umask, as busybox's filters
// it through the process's: under `umask 077`, `chmod +x f` lets only the owner run f.
func TestRuntime_chmodFiltersAModeWithoutClassesThroughTheUmask(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + "'\numask 077\nchmod -v +x f\numask 022\nchmod -v +x f\n"

	status := rt.RunScript(context.Background(), script)

	want := "mode of 'f' changed to 0744 (rwxr--r--)\nmode of 'f' changed to 0755 (rwxr-xr-x)\n"
	if status != 0 || stdout.String() != want || stderr.String() != "" {
		t.Fatalf("got %d, %q, %q; want 0, %q", status, stdout.String(), stderr.String(), want)
	}
}
