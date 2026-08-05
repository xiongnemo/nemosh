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

func TestRuntime_evaluatesArguments_whenEvalRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "eval echo hi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected eval output %q, got %q", "hi\n", got)
	}
}

func TestRuntime_sourcesScriptInCurrentRuntime_whenDotRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "library.sh"))
	if err := os.WriteFile(scriptPath, []byte("name=sourced\n"), 0o600); err != nil {
		t.Fatalf("expected source fixture write to succeed, got %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+"\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "sourced\n" {
		t.Fatalf("expected sourced variable output %q, got %q", "sourced\n", got)
	}
}

func TestRuntime_sourcesScriptInCurrentRuntime_whenSourceRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "library.sh"))
	if err := os.WriteFile(scriptPath, []byte("name=sourced\n"), 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "source "+scriptPath+"\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if got := stdout.String(); got != "sourced\n" {
		t.Fatalf("stdout = %q, want %q", got, "sourced\n")
	}
}

func TestRuntime_sourcesHomeRelativeScript_whenSourceUsesTilde(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".profile"), []byte("name=profile\n"), 0o600); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"HOME=" + home}),
	})

	// When
	status := rt.RunScript(context.Background(), "source ~/.profile\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if got := stdout.String(); got != "profile\n" {
		t.Fatalf("stdout = %q, want %q", got, "profile\n")
	}
}

func TestRuntime_defersExitTrap_whenEvalRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo bye' EXIT\neval echo hi\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\nafter\nbye\n" {
		t.Fatalf("expected deferred trap output %q, got %q", "hi\nafter\nbye\n", got)
	}
}

func TestRuntime_marksAssignedVariable_whenReadonlyRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "readonly name=old\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "old\n" {
		t.Fatalf("expected readonly assignment output %q, got %q", "old\n", got)
	}
}

func TestRuntime_preservesReadonlyVariable_whenAssignmentRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	if status := rt.RunScript(context.Background(), "readonly name=old\n"); status != 0 {
		t.Fatalf("expected readonly setup status 0, got %d", status)
	}

	// When
	status := rt.RunScript(context.Background(), "name=new\n")
	finalStatus := rt.RunScript(context.Background(), "echo $name\n")

	// Then
	if status == 0 {
		t.Fatalf("expected readonly assignment to fail")
	}
	if finalStatus != 0 {
		t.Fatalf("expected final echo status 0, got %d", finalStatus)
	}
	if got := stdout.String(); got != "old\n" {
		t.Fatalf("expected readonly variable to stay %q, got %q", "old\n", got)
	}
}

func TestRuntime_preservesReadonlyVariable_whenExportAssigns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	if status := rt.RunScript(context.Background(), "readonly name=old\n"); status != 0 {
		t.Fatalf("expected readonly setup status 0, got %d", status)
	}

	// When
	status := rt.RunScript(context.Background(), "export name=new\n")
	finalStatus := rt.RunScript(context.Background(), "echo $name\n")

	// Then
	if status == 0 {
		t.Fatalf("expected readonly export assignment to fail")
	}
	if finalStatus != 0 {
		t.Fatalf("expected final echo status 0, got %d", finalStatus)
	}
	if got := stdout.String(); got != "old\n" {
		t.Fatalf("expected readonly variable to stay %q, got %q", "old\n", got)
	}
}

func TestRuntime_preservesReadonlyVariable_whenUnsetRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	if status := rt.RunScript(context.Background(), "readonly name=old\n"); status != 0 {
		t.Fatalf("expected readonly setup status 0, got %d", status)
	}

	// When
	status := rt.RunScript(context.Background(), "unset name\n")
	finalStatus := rt.RunScript(context.Background(), "echo $name\n")

	// Then
	if status == 0 {
		t.Fatalf("expected readonly unset to fail")
	}
	if finalStatus != 0 {
		t.Fatalf("expected final echo status 0, got %d", finalStatus)
	}
	if got := stdout.String(); got != "old\n" {
		t.Fatalf("expected readonly variable to stay %q, got %q", "old\n", got)
	}
}
