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
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_externalLookupDoesNotSearch_whenExactPATHIsMissingOrEmpty(t *testing.T) {
	command := "nemosh-host-path-decoy"
	hostBin := t.TempDir()
	copyRuntimeHelper(t, filepath.Join(hostBin, executableName(command)))
	t.Setenv("PATH", hostBin)

	tests := []struct {
		name string
		env  []string
	}{
		{name: "missing", env: []string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}},
		{name: "empty", env: []string{"PATH=", "NEMOSH_RUNTIME_HELPER_PROCESS=1"}},
		{name: "case variant only", env: []string{"Path=" + hostBin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
				Cwd: runtime.WorkingDirectory(t.TempDir()),
				Env: runtime.NewEnvironment(test.env),
			})

			status := rt.RunScript(context.Background(), command+" -test.run=TestRuntimeHelperProcess -- executable\n")

			if status != 127 {
				t.Fatalf("status: got %d, want 127", status)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout: %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), command+": not found") {
				t.Fatalf("stderr: %q", stderr.String())
			}
		})
	}
}

func TestRuntime_externalLookupUsesRuntimeCwd_whenNonemptyPATHHasEmptyComponent(t *testing.T) {
	command := "nemosh-empty-path-component"
	runtimeDir := t.TempDir()
	runtimeExecutable := filepath.Join(runtimeDir, executableName(command))
	copyRuntimeHelper(t, runtimeExecutable)
	hostBin := t.TempDir()
	copyRuntimeHelper(t, filepath.Join(hostBin, executableName(command)))
	t.Setenv("PATH", hostBin)

	tests := []struct {
		name string
		path string
	}{
		{name: "leading", path: string(os.PathListSeparator) + "missing"},
		{name: "trailing", path: "missing" + string(os.PathListSeparator)},
		{name: "repeated", path: "missing" + strings.Repeat(string(os.PathListSeparator), 2) + "also-missing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
				Cwd: runtime.WorkingDirectory(runtimeDir),
				Env: runtime.NewEnvironment([]string{"PATH=" + test.path, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
			})

			status := rt.RunScript(context.Background(), command+" -test.run=TestRuntimeHelperProcess -- executable\n")

			if status != 0 {
				t.Fatalf("status: got %d, want 0", status)
			}
			if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, runtimeExecutable) {
				t.Fatalf("executable: got %q, want %q", got, runtimeExecutable)
			}
		})
	}
}

func TestRuntime_explicitExternalPathBypassesMissingAndEmptyPATH(t *testing.T) {
	commandPath := filepath.Join(t.TempDir(), executableName("nemosh-explicit-path"))
	copyRuntimeHelper(t, commandPath)

	tests := []struct {
		name string
		env  []string
	}{
		{name: "missing", env: []string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}},
		{name: "empty", env: []string{"PATH=", "NEMOSH_RUNTIME_HELPER_PROCESS=1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
				Cwd: runtime.WorkingDirectory(t.TempDir()),
				Env: runtime.NewEnvironment(test.env),
			})

			status := rt.RunScript(context.Background(), shellSingleQuote(filepath.ToSlash(commandPath))+" -test.run=TestRuntimeHelperProcess -- executable\n")

			if status != 0 {
				t.Fatalf("status: got %d, want 0", status)
			}
			if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, commandPath) {
				t.Fatalf("executable: got %q, want %q", got, commandPath)
			}
		})
	}
}

func TestRuntime_externalLookupUsesExactUppercasePATH_overCaseVariant(t *testing.T) {
	command := "nemosh-path-case-order"
	runtimeBin := t.TempDir()
	decoyBin := t.TempDir()
	runtimeExecutable := filepath.Join(runtimeBin, executableName(command))
	copyRuntimeHelper(t, runtimeExecutable)
	copyRuntimeHelper(t, filepath.Join(decoyBin, executableName(command)))
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"Path=" + decoyBin, "PATH=" + runtimeBin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 0 {
		t.Fatalf("status: got %d, want 0", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, runtimeExecutable) {
		t.Fatalf("executable: got %q, want %q", got, runtimeExecutable)
	}
}

func executableName(command string) string {
	if goruntime.GOOS == "windows" {
		return command + ".exe"
	}
	return command
}

func sameNativePath(left, right string) bool {
	if goruntime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
