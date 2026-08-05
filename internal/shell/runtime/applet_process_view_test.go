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

func TestRuntime_appletsUseRuntimeProcessView_whenStateDiffersFromHost(t *testing.T) {
	// Given
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "input.txt"), []byte("runtime-file\n"), 0o600); err != nil {
		t.Fatalf("write applet fixture: %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(runtimeDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_APPLET_ENV=runtime"}),
	})

	// When
	status := rt.RunScript(context.Background(), "command pwd\nenv NEMOSH_CHILD_ENV=child printenv NEMOSH_CHILD_ENV\ncat input.txt\ntouch created.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	want := displayPath(runtimeDir) + "\nchild\nruntime-file\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected applet output %q, got %q", want, got)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "created.txt")); err != nil {
		t.Fatalf("expected runtime-relative touch: %v", err)
	}
}

func TestApplet_fallsBackToHostProcessView_whenRunStandalone(t *testing.T) {
	// Given
	hostCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get host cwd: %v", err)
	}
	var stdout bytes.Buffer
	applet, ok := applets.DefaultRegistry.Lookup("pwd")
	if !ok {
		t.Fatal("expected pwd applet")
	}

	// When
	err = applet.Run(context.Background(), nil, bytes.NewReader(nil), &stdout, &stdout)

	// Then
	if err != nil {
		t.Fatalf("expected standalone pwd success, got %v", err)
	}
	if got, want := stdout.String(), hostCwd+"\n"; got != want {
		t.Fatalf("expected standalone pwd %q, got %q", want, got)
	}
}

func TestRuntime_envAppletPreservesDistinctExactCaseNames(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment(nil),
	})

	// When
	status := rt.RunScript(context.Background(), "env -i foo=lower FOO=upper printenv foo FOO\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "lower\nupper\n" {
		t.Fatalf("expected runtime and standalone lookup parity, got %q", got)
	}
}

func TestP05WaveA_EnvFilesystemApplet_usesTypedTmpBackingLikeDirectInvocation(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpRoot, "input.txt"), []byte("typed-backing\n"), 0o600); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	settings := runtime.DefaultPathSettings()
	settings.TmpRoot = runtime.WorkingDirectory(tmpRoot)
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd:   runtime.WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(), "cat /tmp/input.txt\nenv CHILD=value cat /tmp/input.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected direct/env typed alias parity, status=%d stderr=%q", status, stderr.String())
	}
	if got := stdout.String(); got != "typed-backing\ntyped-backing\n" {
		t.Fatalf("expected direct/env backing output parity, got %q", got)
	}
}

func TestP05WaveA_EnvFilesystemApplet_propagatesTypedPathErrorBeforeEffect(t *testing.T) {
	// Given
	cwd := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd: runtime.WorkingDirectory(cwd),
	})

	// When
	status := rt.RunScript(context.Background(), "env CHILD=value touch /cygdrive/c/nemosh-env-overlay-effect\n")

	// Then
	if status == 0 {
		t.Fatal("expected disabled Cygdrive path to fail")
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected no misleading success output, got %q", got)
	}
	if got := stderr.String(); !strings.Contains(got, "cygdrive paths are disabled") {
		t.Fatalf("expected typed pathmodel error, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(cwd, "nemosh-env-overlay-effect")); !os.IsNotExist(err) {
		t.Fatalf("expected no filesystem effect, stat error=%v", err)
	}
}

func TestP05WaveA_EnvThenXargs_preservesTypedTmpBackingTransitively(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpRoot, "input.txt"), []byte("nested-backing\n"), 0o600); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	settings := runtime.DefaultPathSettings()
	settings.TmpRoot = runtime.WorkingDirectory(tmpRoot)
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{
		Stdin:  strings.NewReader("/tmp/input.txt\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	}, runtime.State{Cwd: runtime.WorkingDirectory(t.TempDir()), Paths: &settings})

	// When
	status := rt.RunScript(context.Background(), "env CHILD=value xargs cat\n")

	// Then
	if status != 0 {
		t.Fatalf("expected nested wrapper typed path success, status=%d stderr=%q", status, stderr.String())
	}
	if got := stdout.String(); got != "nested-backing\n" {
		t.Fatalf("expected nested backing output, got %q", got)
	}
}
