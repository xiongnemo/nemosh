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

func TestRuntime_executesAppletCommand_whenScriptContainsEcho(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected stdout %q, got %q", "hi\n", got)
	}
}

func TestRuntime_changesDirectory_whenScriptContainsCdAndPwd(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "cd /\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() == "" {
		t.Fatal("expected pwd output")
	}
}

func TestRuntime_stopsWithStatus_whenScriptExits(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "exit 7\necho unreachable\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
}

func TestRuntime_expandsVariable_whenAssignmentPrecedesEcho(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "name=nemo\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "nemo\n", got)
	}
}

func TestRuntime_runsAndOrLists_withShortCircuit(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false && echo bad\ntrue || echo bad\necho ok\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "ok\n" {
		t.Fatalf("expected stdout %q, got %q", "ok\n", got)
	}
}

func TestRuntime_redirectsStdout_whenCommandUsesOutputRedirection(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	tmpDir := t.TempDir()
	outputPath := filepath.ToSlash(filepath.Join(tmpDir, "out.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo hi > "+outputPath+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected stdout to be empty, got %q", stdout.String())
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected redirected output file, got %v", err)
	}
	if string(contents) != "hi\n" {
		t.Fatalf("expected redirected contents %q, got %q", "hi\n", string(contents))
	}
}

func TestRuntime_redirectsStdin_whenCommandUsesInputRedirection(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	tmpDir := t.TempDir()
	inputPath := filepath.ToSlash(filepath.Join(tmpDir, "in.txt"))
	if err := os.WriteFile(inputPath, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "cat < "+inputPath+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "from-file\n" {
		t.Fatalf("expected stdout %q, got %q", "from-file\n", got)
	}
}
