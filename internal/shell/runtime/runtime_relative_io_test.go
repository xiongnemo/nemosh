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

func TestRuntime_resolvesSourceAgainstRuntimeCwd_whenOperandIsRelative(t *testing.T) {
	// Given
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "setup.sh"), []byte("value=from-source\n"), 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{Cwd: runtime.WorkingDirectory(runtimeDir), Env: runtime.NewEnvironment(nil)})

	// When
	status := rt.RunScript(context.Background(), ". setup.sh\necho $value\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "from-source\n" {
		t.Fatalf("expected sourced output %q, got %q", "from-source\n", got)
	}
}

func TestRuntime_resolvesRedirectionsAgainstRuntimeCwd_whenOperandsAreRelative(t *testing.T) {
	// Given
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "input.txt"), []byte("runtime-input\n"), 0o600); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{Cwd: runtime.WorkingDirectory(runtimeDir), Env: runtime.NewEnvironment(nil)})

	// When
	status := rt.RunScript(context.Background(), "cat < input.txt\necho runtime-output > output.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "runtime-input\n" {
		t.Fatalf("expected redirected input %q, got %q", "runtime-input\n", got)
	}
	contents, err := os.ReadFile(filepath.Join(runtimeDir, "output.txt"))
	if err != nil {
		t.Fatalf("read redirected output: %v", err)
	}
	if got := string(contents); got != "runtime-output\n" {
		t.Fatalf("expected redirected output %q, got %q", "runtime-output\n", got)
	}
}
