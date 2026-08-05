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

func TestRuntime_usesIndependentExecutionState_whenRuntimesDiffer(t *testing.T) {
	// Given
	hostCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get host cwd: %v", err)
	}
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	var firstOut bytes.Buffer
	var secondOut bytes.Buffer
	first := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &firstOut}, runtime.State{
		Cwd: runtime.WorkingDirectory(firstDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_RUNTIME_VALUE=first", "NEMOSH_EMPTY="}),
	})
	second := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &secondOut}, runtime.State{
		Cwd: runtime.WorkingDirectory(secondDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_RUNTIME_VALUE=second"}),
	})

	// When
	firstStatus := first.RunScript(context.Background(), "pwd\nprintenv NEMOSH_RUNTIME_VALUE NEMOSH_EMPTY\n")
	secondStatus := second.RunScript(context.Background(), "pwd\nprintenv NEMOSH_RUNTIME_VALUE\n")

	// Then
	if firstStatus != 0 || secondStatus != 0 {
		t.Fatalf("expected statuses 0, got first=%d second=%d", firstStatus, secondStatus)
	}
	if got, want := firstOut.String(), displayPath(firstDir)+"\nfirst\n\n"; got != want {
		t.Fatalf("expected first output %q, got %q", want, got)
	}
	if got, want := secondOut.String(), displayPath(secondDir)+"\nsecond\n"; got != want {
		t.Fatalf("expected second output %q, got %q", want, got)
	}
	assertHostCwd(t, hostCwd)
}

func TestCommandSubstitution_isolatesCwdAndEnvironment_andPreservesHost(t *testing.T) {
	// Given
	hostCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get host cwd: %v", err)
	}
	hostValue, hostPresent := os.LookupEnv("NEMOSH_SUBSTITUTION_ENV")
	parentDir := t.TempDir()
	childDir := t.TempDir()
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(parentDir),
		Env: runtime.NewEnvironment([]string{"NEMOSH_SUBSTITUTION_ENV=parent"}),
	})
	script := "echo $(cd " + filepath.ToSlash(childDir) + "\npwd)\n" +
		"echo $(export NEMOSH_SUBSTITUTION_ENV=child\nprintenv NEMOSH_SUBSTITUTION_ENV)\n" +
		"echo $(unset NEMOSH_SUBSTITUTION_ENV\nprintenv NEMOSH_SUBSTITUTION_ENV)\n" +
		"pwd\nprintenv NEMOSH_SUBSTITUTION_ENV\n"

	// When
	status := rt.RunScript(context.Background(), script)

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	want := displayPath(childDir) + "\nchild\n\n" + displayPath(parentDir) + "\nparent\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected output %q, got %q", want, got)
	}
	assertHostCwd(t, hostCwd)
	if got, present := os.LookupEnv("NEMOSH_SUBSTITUTION_ENV"); got != hostValue || present != hostPresent {
		t.Fatalf("host environment changed: value=%q present=%t", got, present)
	}
}

func displayPath(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

func assertHostCwd(t *testing.T, want string) {
	t.Helper()
	got, err := os.Getwd()
	if err != nil {
		t.Fatalf("get host cwd after runtime: %v", err)
	}
	if got != want {
		t.Fatalf("host cwd changed: want %q, got %q", want, got)
	}
}
