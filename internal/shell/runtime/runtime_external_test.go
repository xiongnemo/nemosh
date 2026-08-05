package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_executesExternalCommand_whenCommandIsNotBuiltinOrApplet(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(exe)+" -test.run=TestRuntimeHelperProcess -- external-ok\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "external-ok\n" {
		t.Fatalf("expected external stdout %q, got %q", "external-ok\n", got)
	}
}

func TestRuntime_returnsExternalCommandStatus_whenCommandExitsNonZero(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(exe)+" -test.run=TestRuntimeHelperProcess -- exit-7\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
}

func TestRuntime_usesFixedWindowsSuffixOrder_whenExternalCommandHasNoSuffix(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("fixed executable suffix order is Windows-only")
	}
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	dir := t.TempDir()
	command := "nemosh-suffix-probe"
	for _, suffix := range []string{".com", ".exe"} {
		candidate := filepath.Join(dir, command+suffix)
		data, readErr := os.ReadFile(exe)
		if readErr != nil {
			t.Fatalf("expected test executable read to succeed, got %v", readErr)
		}
		if writeErr := os.WriteFile(candidate, data, 0o700); writeErr != nil {
			t.Fatalf("expected candidate %s write to succeed, got %v", suffix, writeErr)
		}
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".EXE;.COM")
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := filepath.Ext(strings.TrimSpace(stdout.String())); got != ".com" {
		t.Fatalf("expected fixed suffix order to choose .com, got extension %q from output %q", got, stdout.String())
	}
}

func TestRuntime_usesUnsuffixedExecutableFromRuntimeRelativePath_withoutHostPathFallback_onNonWindows(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Unix executable lookup semantics are non-Windows-only")
	}
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	runtimeDir := t.TempDir()
	runtimeBin := filepath.Join(runtimeDir, "bin")
	hostBin := t.TempDir()
	if err := os.Mkdir(runtimeBin, 0o700); err != nil {
		t.Fatalf("expected runtime bin creation to succeed, got %v", err)
	}
	command := "nemosh-runtime-path-probe"
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("expected test executable read to succeed, got %v", err)
	}
	for _, candidate := range []string{filepath.Join(runtimeBin, command), filepath.Join(hostBin, command)} {
		if err := os.WriteFile(candidate, data, 0o700); err != nil {
			t.Fatalf("expected helper command write to succeed, got %v", err)
		}
	}
	t.Setenv("PATH", hostBin)
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(runtimeDir),
		Env: runtime.NewEnvironment([]string{"PATH=bin", "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	// When
	status := rt.RunScript(context.Background(), command+" -test.run=TestRuntimeHelperProcess -- executable\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := strings.TrimSpace(stdout.String()); got != filepath.Join(runtimeBin, command) {
		t.Fatalf("expected runtime-owned executable %q, got %q", filepath.Join(runtimeBin, command), got)
	}
}

func TestRuntime_externalChildUsesRuntimeCwdEnvPathAndTemporaryAssignments(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("get test executable: %v", err)
	}
	runtimeDir := t.TempDir()
	commandDir := filepath.Join(runtimeDir, "bin")
	if err := os.Mkdir(commandDir, 0o700); err != nil {
		t.Fatalf("create command dir: %v", err)
	}
	command := "nemosh-runtime-state"
	candidate := filepath.Join(commandDir, command)
	if goruntime.GOOS == "windows" {
		candidate += ".exe"
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("read test executable: %v", err)
	}
	if err := os.WriteFile(candidate, data, 0o700); err != nil {
		t.Fatalf("write helper command: %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(runtimeDir),
		Env: runtime.NewEnvironment([]string{"PATH=bin", "NEMOSH_RUNTIME_HELPER_PROCESS=1", "NEMOSH_CHILD_VALUE=persistent"}),
	})

	// When
	status := rt.RunScript(context.Background(), "NEMOSH_CHILD_VALUE=temporary "+command+" -test.run=TestRuntimeHelperProcess -- state\nprintenv NEMOSH_CHILD_VALUE\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	want := runtimeDir + "\ntemporary\npersistent\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected child state %q, got %q", want, got)
	}
}

func TestRuntime_windowsChildUsesCanonicalPATHWithoutChangingExactParentEntries(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("native Windows environment serialization")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	orders := [][]string{
		{"PATH=parent-upper", "Path=parent-title", "NEMOSH_RUNTIME_HELPER_PROCESS=1"},
		{"Path=parent-title", "PATH=parent-upper", "NEMOSH_RUNTIME_HELPER_PROCESS=1"},
	}
	for index, items := range orders {
		t.Run(fmt.Sprintf("order-%d", index), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
				Cwd: runtime.WorkingDirectory(t.TempDir()),
				Env: runtime.NewEnvironment(items),
			})
			command := filepath.ToSlash(exe) + " -test.run=TestRuntimeHelperProcess -- env-class PATH"
			status := rt.RunScript(context.Background(), "pAtH=child "+command+"\n")
			if status != 0 {
				t.Fatalf("status: %d", status)
			}
			if got := stdout.String(); got != "PATH=parent-upper\n" {
				t.Fatalf("child class: %q", got)
			}
			if value, ok := rt.LookupEnv("PATH"); !ok || value != "parent-upper" {
				t.Fatalf("parent PATH: value=%q present=%t", value, ok)
			}
			if value, ok := rt.LookupEnv("Path"); !ok || value != "parent-title" {
				t.Fatalf("parent Path: value=%q present=%t", value, ok)
			}
		})
	}
}

func TestRuntime_externalLookupUsesExactPATHWithoutPathFallback(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	dir := t.TempDir()
	command := "nemosh-path-case-probe"
	candidate := filepath.Join(dir, command)
	if goruntime.GOOS == "windows" {
		candidate += ".exe"
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("read executable: %v", err)
	}
	if err := os.WriteFile(candidate, data, 0o700); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"Path=" + dir}),
	})

	if status := rt.RunScript(context.Background(), command+"\n"); status != 127 {
		t.Fatalf("status: %d", status)
	}
}
