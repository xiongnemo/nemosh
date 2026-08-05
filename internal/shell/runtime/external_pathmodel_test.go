package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestP05WaveA_ExternalLookup_searchesRuntimePATHForExplicitSuffix(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows suffix lookup")
	}

	// Given
	runtimeDir := t.TempDir()
	binDir := filepath.Join(runtimeDir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("create bin directory: %v", err)
	}
	copyRuntimeHelper(t, filepath.Join(binDir, "path-probe.exe"))
	var stdout bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout}, shellruntime.State{
		Cwd: shellruntime.WorkingDirectory(runtimeDir),
		Env: shellruntime.NewEnvironment([]string{"PATH=bin", "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	// When
	status := rt.RunScript(context.Background(), "path-probe.exe -test.run=TestRuntimeHelperProcess -- external-ok\n")

	// Then
	if status != 0 || stdout.String() != "external-ok\n" {
		t.Fatalf("expected PATH executable success, status=%d stdout=%q", status, stdout.String())
	}
}

func TestP05WaveA_ExternalLookup_acceptsMountAliasForExplicitExecutable(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows mount aliases")
	}

	// Given
	executable := filepath.Join(t.TempDir(), "mount-probe.exe")
	copyRuntimeHelper(t, executable)
	mountExecutable := "/mnt" + displayPath(executable)
	var stdout bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout}, shellruntime.State{
		Cwd: shellruntime.WorkingDirectory(t.TempDir()),
		Env: shellruntime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	// When
	status := rt.RunScript(context.Background(), mountExecutable+" -test.run=TestRuntimeHelperProcess -- external-ok\n")

	// Then
	if status != 0 || stdout.String() != "external-ok\n" {
		t.Fatalf("expected mount-alias executable success, status=%d stdout=%q", status, stdout.String())
	}
}

func TestP05WaveA_ExternalLaunch_preservesOrdinaryArgvWithoutPathConversion(t *testing.T) {
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("get test executable: %v", err)
	}
	var stdout bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout}, shellruntime.State{
		Cwd: shellruntime.WorkingDirectory(t.TempDir()),
		Env: shellruntime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})
	args := []string{"/mnt/c/not-a-path", "/c/still-plain-data"}

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(executable)+" -test.run=TestRuntimeHelperProcess -- argv "+strings.Join(args, " ")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected helper status 0, got %d", status)
	}
	if got, want := stdout.String(), strings.Join(args, "\n")+"\n"; got != want {
		t.Fatalf("expected argv %q, got %q", want, got)
	}
}

func TestP05WaveA_ExternalLookup_reportsPathmodelErrorBeforeLaunch(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows Cygdrive policy")
	}

	// Given
	var stderr bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stderr: &stderr}, shellruntime.State{
		Cwd: shellruntime.WorkingDirectory(t.TempDir()),
	})

	// When
	status := rt.RunScript(context.Background(), "/cygdrive/c/missing.exe\n")

	// Then
	if status == 0 {
		t.Fatal("expected pathmodel lookup failure")
	}
	if output := stderr.String(); !strings.Contains(output, "cygdrive paths are disabled") || strings.Contains(output, "not found") {
		t.Fatalf("expected pathmodel error instead of not-found, got %q", output)
	}
}

func copyRuntimeHelper(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatalf("get test executable: %v", err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read test executable: %v", err)
	}
	if err := os.WriteFile(destination, data, 0o700); err != nil {
		t.Fatalf("write helper executable: %v", err)
	}
}
