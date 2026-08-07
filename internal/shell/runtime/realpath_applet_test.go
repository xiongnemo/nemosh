package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_realpathPrintsAbsolutePath_whenOperandExists(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := writeRuntimeRealpathFixture(t, canonicalTempDir(t), "file.txt")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "realpath "+filepath.ToSlash(path)+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got, want := stdout.String(), slashRuntimeAbs(t, path)+"\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_realpathFailsAndContinues_whenOneOperandParentIsMissing(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := canonicalTempDir(t)
	first := writeRuntimeRealpathFixture(t, dir, "first.txt")
	missing := filepath.Join(dir, "missing-parent", "file.txt")
	second := writeRuntimeRealpathFixture(t, dir, "second.txt")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "realpath "+filepath.ToSlash(first)+" "+filepath.ToSlash(missing)+" "+filepath.ToSlash(second)+"\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	wantStdout := slashRuntimeAbs(t, first) + "\n" + slashRuntimeAbs(t, second) + "\n"
	if got := stdout.String(); got != wantStdout {
		t.Fatalf("expected successful operands stdout %q, got %q", wantStdout, got)
	}
	if got := stderr.String(); !strings.Contains(got, "realpath:") || !strings.Contains(got, filepath.ToSlash(missing)) {
		t.Fatalf("expected realpath diagnostic for %q, got %q", filepath.ToSlash(missing), got)
	}
}

func writeRuntimeRealpathFixture(t *testing.T, dir string, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("expected fixture directory creation to succeed, got %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("realpath"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	return path
}

func slashRuntimeAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("expected absolute path for %q, got %v", path, err)
	}
	return displayPath(filepath.Clean(abs))
}

// canonicalTempDir is t.TempDir() resolved the way the path model and realpath
// resolve it, so a test can use the same spelling for the file it creates and
// for the answer it expects.
//
// They differ on a machine whose TEMP sits under an 8.3 alias: GitHub's Windows
// runners hand out `C:\Users\RUNNER~1\AppData\Local\Temp`, because the profile
// directory name is longer than eight characters. There t.TempDir() is the short
// spelling while realpath answers with the long one, and a test comparing the
// two fails for a reason that has nothing to do with what it is testing.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}
