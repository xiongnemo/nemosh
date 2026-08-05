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

func TestRuntime_routesOutputThroughDevFDNumberedAliases(t *testing.T) {
	tmp := t.TempDir()
	fd3 := filepath.ToSlash(filepath.Join(tmp, "fd3.txt"))
	fd49 := filepath.ToSlash(filepath.Join(tmp, "fd49.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	status := rt.RunScript(context.Background(), "echo three 3>"+fd3+" 1>/dev/fd/3\necho forty-nine 49>"+fd49+" 1>/dev/fd/49\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, fd3, "three\n")
	assertFileText(t, fd49, "forty-nine\n")
}

func TestRuntime_devStdoutCapturesBindingBeforeLaterRebind(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	tmp := t.TempDir()
	first := filepath.ToSlash(filepath.Join(tmp, "first.txt"))
	second := filepath.ToSlash(filepath.Join(tmp, "second.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	probe := filepath.ToSlash(exe) + " -test.run=TestRedirectHelperProcess -- alias"
	status := rt.RunScript(context.Background(), probe+" 1>"+first+" 3>/dev/stdout 1>"+second+" 2>&3\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, first, "alias\n")
	assertFileText(t, second, "")
}

func TestRuntime_rejectsMalformedClosedAndDirectionMismatchedDevFD(t *testing.T) {
	tests := []string{
		"echo bad >/dev/fd/",
		"echo bad >/dev/fd/+1",
		"echo bad >/dev/fd/x",
		"echo bad >/dev/fd/256",
		"echo bad 3>&- >/dev/fd/3",
		"echo bad 3</dev/null 1>/dev/fd/3",
	}
	for _, script := range tests {
		t.Run(script, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
			if status := rt.RunScript(context.Background(), script+"\n"); status == 0 {
				t.Fatal("expected device alias failure")
			}
			if stdout.Len() != 0 {
				t.Fatalf("command executed: %q", stdout.String())
			}
		})
	}
}

func TestCommandSubstitution_capturesChildStderrDupAndPreservesParent(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	probe := filepath.ToSlash(exe) + " -test.run=TestRedirectHelperProcess -- child 2>&1"

	status := rt.RunScript(context.Background(), "echo $("+probe+")\necho parent\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got, want := stdout.String(), "child\nparent\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("parent stderr changed: %q", stderr.String())
	}
}
