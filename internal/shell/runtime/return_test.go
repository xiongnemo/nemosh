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

func TestRuntime_returnsFromDotScriptWithStatus_whenReturnRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	scriptPath := writeReturnScript(t, "echo before\nreturn 7\necho unreachable\n")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+"\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected final status 0, got %d", status)
	}
	if got := stdout.String(); got != "before\nafter\n" {
		t.Fatalf("expected return output %q, got %q", "before\nafter\n", got)
	}
}

func TestRuntime_usesReturnStatusForDotCommand_whenReturnRunsInOrList(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	scriptPath := writeReturnScript(t, "return 7\necho unreachable\n")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+" || echo recovered\n")

	// Then
	if status != 0 {
		t.Fatalf("expected final status 0, got %d", status)
	}
	if got := stdout.String(); got != "recovered\n" {
		t.Fatalf("expected recovered output %q, got %q", "recovered\n", got)
	}
}

func TestRuntime_continuesTopLevelScript_whenReturnRunsOutsideDotScript(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "return 7\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected final status 0, got %d", status)
	}
	if got := stdout.String(); got != "after\n" {
		t.Fatalf("expected top-level return output %q, got %q", "after\n", got)
	}
	if stderr.String() == "" {
		t.Fatalf("expected top-level return diagnostic")
	}
}

func writeReturnScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "library.sh"))
	if err := os.WriteFile(scriptPath, []byte(content), 0o600); err != nil {
		t.Fatalf("expected return fixture write to succeed, got %v", err)
	}
	return scriptPath
}
