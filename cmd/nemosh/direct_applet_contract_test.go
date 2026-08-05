package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestDirectApplet_preservesProcessInputs_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	t.Setenv("NEMOSH_DIRECT_APPLET_TEST", "environment-value")
	tests := []struct {
		name, stdin, stdout string
		args                []string
	}{
		{name: "explicit argv", args: []string{"nemosh", "echo", "hello", "world"}, stdout: "hello world\n"},
		{name: "multicall argv", args: []string{directAppletInvocationName("echo"), "hello", "world"}, stdout: "hello world\n"},
		{name: "explicit stdin", args: []string{"nemosh", "cat"}, stdin: "stream-data", stdout: "stream-data"},
		{name: "multicall stdin", args: []string{directAppletInvocationName("cat")}, stdin: "stream-data", stdout: "stream-data"},
		{name: "explicit environment", args: []string{"nemosh", "printenv", "NEMOSH_DIRECT_APPLET_TEST"}, stdout: "environment-value\n"},
		{name: "multicall environment", args: []string{directAppletInvocationName("printenv"), "NEMOSH_DIRECT_APPLET_TEST"}, stdout: "environment-value\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			cmd := command{stdin: bytes.NewBufferString(test.stdin), stdout: &stdout, stderr: &stderr}

			// When
			err := cmd.run(context.Background(), test.args)

			// Then
			if err != nil || stdout.String() != test.stdout || stderr.Len() != 0 {
				t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", test.args, stdout.String(), stderr.String(), err)
			}
		})
	}
}

func TestDirectApplet_preservesStatusAndStderr_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "sort", "-z"}, {directAppletInvocationName("sort"), "-z"}} {
		// Given
		var stdout, stderr bytes.Buffer
		cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr}

		// When
		err := cmd.run(context.Background(), args)

		// Then
		status, ok := applets.StatusCode(err)
		if !ok || status != 2 || stdout.Len() != 0 || stderr.String() != "sort: invalid option -- z\n" {
			t.Fatalf("run(%v): status=(%d,%t) stdout=%q stderr=%q error=%v", args, status, ok, stdout.String(), stderr.String(), err)
		}
	}
}

func TestDirectApplet_touchUsesInjectedRelativeCWDForBothEntryForms(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "touch", "relative.txt"}, {directAppletInvocationName("touch"), "relative.txt"}} {
		// Given
		cwd := t.TempDir()
		state := shellruntime.State{Cwd: shellruntime.WorkingDirectory(cwd), Env: shellruntime.NewEnvironment(os.Environ())}

		// When
		stdout, stderr, err := runDirectAppletStateTest(args, state)

		// Then
		if err != nil || stdout != "" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
		if _, statErr := os.Stat(filepath.Join(cwd, "relative.txt")); statErr != nil {
			t.Fatalf("relative effect: %v", statErr)
		}
	}
}

func TestDirectApplet_rejectsInvalidInjectedStateBeforeEffectsForBothEntryForms(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "echo", "effect"}, {directAppletInvocationName("echo"), "effect"}} {
		stdout, stderr, err := runDirectAppletStateTest(args, shellruntime.State{})

		if !errors.Is(err, shellruntime.ErrStateCwdRequired) {
			t.Fatalf("run(%v): got error %v, want %v", args, err, shellruntime.ErrStateCwdRequired)
		}
		if stdout != "" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q", args, stdout, stderr)
		}
	}
}

func TestDirectApplet_rejectsRelativeTmpRootBeforeEffectsForBothEntryForms(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "touch", "/tmp/effect"}, {directAppletInvocationName("touch"), "/tmp/effect"}} {
		t.Run(args[0], func(t *testing.T) {
			hostCwd := t.TempDir()
			backing := filepath.Join(hostCwd, "relative-tmp")
			if err := os.Mkdir(backing, 0o700); err != nil {
				t.Fatalf("create relative tmp backing: %v", err)
			}
			t.Chdir(hostCwd)
			settings := shellruntime.DefaultPathSettings()
			settings.TmpRoot = "relative-tmp"
			state := shellruntime.State{
				Cwd:   shellruntime.WorkingDirectory(t.TempDir()),
				Paths: &settings,
			}

			stdout, stderr, err := runDirectAppletStateTest(args, state)

			if !errors.Is(err, shellruntime.ErrStateTmpRootNotAbsolute) {
				t.Fatalf("run(%v): got error %v, want %v", args, err, shellruntime.ErrStateTmpRootNotAbsolute)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("run(%v): stdout=%q stderr=%q", args, stdout, stderr)
			}
			if _, statErr := os.Stat(filepath.Join(backing, "effect")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("run(%v): effect path error=%v, want not exist", args, statErr)
			}
		})
	}
}

func TestDirectApplet_clearsInterruptController_whenInjectedStateIsInvalid(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("echo")
	if !ok {
		t.Fatal("echo applet not found")
	}
	controller := &interruptController{}
	state := shellruntime.State{}
	cmd := command{stdin: &bytes.Buffer{}, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, state: &state}

	err := cmd.runDirectApplet(context.Background(), controller, applet, []string{"effect"})

	if !errors.Is(err, shellruntime.ErrStateCwdRequired) {
		t.Fatalf("run direct applet: got %v, want %v", err, shellruntime.ErrStateCwdRequired)
	}
	controller.mu.Lock()
	active := controller.active
	controller.mu.Unlock()
	if active != nil {
		t.Fatal("interrupt controller retained active execution after constructor failure")
	}
}

func directAppletInvocationName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func runDirectAppletTest(args []string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), args)
	return stdout.String(), stderr.String(), err
}

func runDirectAppletStateTest(args []string, state shellruntime.State) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr, state: &state}
	err := cmd.run(context.Background(), args)
	return stdout.String(), stderr.String(), err
}
