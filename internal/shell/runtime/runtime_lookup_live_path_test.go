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

func TestRuntime_externalLookupUsesLiveExportedPATH_afterStandaloneAssignment(t *testing.T) {
	command := "nemosh-live-path"
	oldBin := t.TempDir()
	newBin := t.TempDir()
	newExecutable := filepath.Join(newBin, executableName(command))
	copyRuntimeHelper(t, filepath.Join(oldBin, executableName(command)))
	copyRuntimeHelper(t, newExecutable)
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"PATH=" + oldBin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), "PATH="+shellSingleQuote(filepath.ToSlash(newBin))+"\n"+command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 0 {
		t.Fatalf("status: got %d, want 0", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, newExecutable) {
		t.Fatalf("executable: got %q, want %q", got, newExecutable)
	}
}

func TestRuntime_externalLookupUsesLiveUnexportedPATH_afterStandaloneAssignment(t *testing.T) {
	command := "nemosh-live-unexported-path"
	runtimeDir := t.TempDir()
	bin := filepath.Join(runtimeDir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	executable := filepath.Join(bin, executableName(command))
	copyRuntimeHelper(t, executable)
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(runtimeDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), "PATH=bin\n"+command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 0 {
		t.Fatalf("status: got %d, want 0", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, executable) {
		t.Fatalf("executable: got %q, want %q", got, executable)
	}
}

func TestRuntime_externalLookupUsesTemporaryPATH_withoutPersistingIt(t *testing.T) {
	command := "nemosh-temporary-path"
	runtimeDir := t.TempDir()
	bin := filepath.Join(runtimeDir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	executable := filepath.Join(bin, executableName(command))
	copyRuntimeHelper(t, executable)
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd: runtime.WorkingDirectory(runtimeDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), "PATH=bin "+command+" -test.run=TestRuntimeHelperProcess -- executable\n"+command+"\n")

	if status != 127 {
		t.Fatalf("status: got %d, want 127", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, executable) {
		t.Fatalf("temporary executable: got %q, want %q", got, executable)
	}
	if !strings.Contains(stderr.String(), command+": not found") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestRuntime_externalLookupStops_afterLiveExportedPATHBecomesEmpty(t *testing.T) {
	command := "nemosh-cleared-path"
	oldBin := t.TempDir()
	copyRuntimeHelper(t, filepath.Join(oldBin, executableName(command)))
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"PATH=" + oldBin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), "PATH=\n"+command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 127 {
		t.Fatalf("status: got %d, want 127", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), command+": not found") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestRuntime_valuelessExportPreservesImportedPATH(t *testing.T) {
	command := "nemosh-exported-path"
	bin := t.TempDir()
	executable := filepath.Join(bin, executableName(command))
	copyRuntimeHelper(t, executable)
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"PATH=" + bin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), "export PATH\n"+command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 0 {
		t.Fatalf("status: got %d, want 0", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, executable) {
		t.Fatalf("executable: got %q, want %q", got, executable)
	}
}

func TestRuntime_externalStartFailureReportsExecutableError_notNotFound(t *testing.T) {
	executable := filepath.Join(t.TempDir(), executableName("nemosh-invalid-executable"))
	if err := os.WriteFile(executable, []byte("not an executable format"), 0o700); err != nil {
		t.Fatalf("write invalid executable: %v", err)
	}
	var stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment(nil),
	})

	status := rt.RunScript(context.Background(), shellSingleQuote(filepath.ToSlash(executable))+"\n")

	if status != 126 {
		t.Fatalf("status: got %d, want 126", status)
	}
	if stderr.Len() == 0 || strings.Contains(stderr.String(), ": not found") {
		t.Fatalf("stderr: got %q, want actionable non-not-found diagnostic", stderr.String())
	}
}
