package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_snapshotPreservesMutationLedgerIndependently(t *testing.T) {
	runtime := New(applets.DefaultRegistry, Streams{})
	runtime.mutatedVars = map[string]struct{}{"PARENT": {}}

	child, err := runtime.snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	child.mutatedVars["CHILD"] = struct{}{}

	if _, ok := child.mutatedVars["PARENT"]; !ok {
		t.Fatal("snapshot lost parent mutation")
	}
	if _, ok := runtime.mutatedVars["CHILD"]; ok {
		t.Fatal("snapshot mutation leaked to parent")
	}
	if err := child.fds.closeAll(); err != nil {
		t.Fatalf("close child: %v", err)
	}
}

func TestRuntime_temporaryAssignmentWithRedirectMergesBuiltinExportOnly(t *testing.T) {
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "redirect.txt"))

	status := runtime.RunScript(context.Background(), "X=parent\nX=temporary command export Y=kept >"+path+"\necho $X:$Y\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got := stdout.String(); got != "parent:kept\n" {
		t.Fatalf("stdout: %q", got)
	}
}

func TestRuntime_redirectedBuiltinRecordsMutationInSharedLedger(t *testing.T) {
	runtime := New(applets.DefaultRegistry, Streams{})
	runtime.mutatedVars = make(map[string]struct{})

	status := runtime.runCommandWithRedirectOperations(context.Background(), []shellToken{{kind: tokenWord, value: "command"}, {kind: tokenWord, value: "export"}, {kind: tokenWord, value: "Y=kept"}}, nil)

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if _, ok := runtime.mutatedVars["Y"]; !ok {
		t.Fatal("redirect execution lost mutation ledger")
	}
}

func TestRuntime_localAssignmentObservesRedirectedBuiltinMutation(t *testing.T) {
	runtime := New(applets.DefaultRegistry, Streams{})
	commandRuntime := runtime.withLocalAssignments([]assignment{{name: "X", value: "temporary"}})
	if commandRuntime == nil {
		t.Fatal("local assignment failed")
	}

	status := commandRuntime.runCommandWithRedirectOperations(context.Background(), []shellToken{{kind: tokenWord, value: "command"}, {kind: tokenWord, value: "export"}, {kind: tokenWord, value: "Y=kept"}}, nil)

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if _, ok := commandRuntime.mutatedVars["Y"]; !ok {
		t.Fatal("local runtime lost mutation")
	}
	if value, ok := commandRuntime.env.LookupEnv("Y"); !ok || value != "kept" {
		t.Fatalf("local environment: value=%q present=%t", value, ok)
	}
}

func TestRuntime_tokenAssignmentMergesRedirectedBuiltinMutation(t *testing.T) {
	runtime := New(applets.DefaultRegistry, Streams{})
	runtime.vars["X"] = "parent"
	path := t.TempDir() + "/redirect.txt"
	tokens := []shellToken{{kind: tokenWord, value: "X=temporary"}, {kind: tokenWord, value: "command"}, {kind: tokenWord, value: "export"}, {kind: tokenWord, value: "Y=kept"}}
	operations := []redirectOperation{{kind: redirectOutput, target: 1, path: path}}

	status := runtime.runCommandWithTokenAssignments(context.Background(), tokens, operations)

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if value := runtime.vars["Y"]; value != "kept" {
		t.Fatalf("merged value: %q", value)
	}
}

func TestRuntime_withStreamsReleasesTableAfterRebindFailure(t *testing.T) {
	resource := &countingReadWriteCloser{}
	table := newFDTable(Streams{})
	if err := table.bindOwned(3, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}
	released := newBorrowedDescription(bytes.NewBuffer(nil), nil)
	if err := released.release(); err != nil {
		t.Fatalf("release fixture: %v", err)
	}
	table.entries[0] = &fdEntry{description: released, capability: readable}
	runtime := New(applets.DefaultRegistry, Streams{}).withFDTable(table)

	_, err := runtime.withStreams(Streams{})

	if !errors.Is(err, errDescriptionReleased) {
		t.Fatalf("with streams: %v", err)
	}
	if resource.closes != 1 {
		t.Fatalf("close count: %d", resource.closes)
	}
}

func TestEnvironment_zeroValueIsUsableAndCloneable(t *testing.T) {
	var environment Environment
	environment.Set("Path", "")
	clone := environment.clone()
	clone.Set("PATH", "second")

	if value, ok := environment.LookupEnv("Path"); !ok || value != "" {
		t.Fatalf("Path: value=%q present=%t", value, ok)
	}
	if got, want := clone.childEnviron(windowsEnvironment), []string{"PATH=second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("child env: got %v want %v", got, want)
	}
	environment.Unset("Path")
	if got := environment.Environ(); len(got) != 0 {
		t.Fatalf("environment: %v", got)
	}
}

func TestNewRuntimeWithState_rejectsMissingAndRelativeCwd(t *testing.T) {
	tests := []struct {
		name string
		cwd  WorkingDirectory
		err  error
	}{
		{name: "missing", err: ErrStateCwdRequired},
		{name: "dot", cwd: ".", err: ErrStateCwdNotAbsolute},
		{name: "relative", cwd: "work", err: ErrStateCwdNotAbsolute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Cwd: test.cwd})

			if !errors.Is(err, test.err) {
				t.Fatalf("construct runtime: got %v, want %v", err, test.err)
			}
		})
	}
}

func TestNewRuntimeWithState_acceptsAbsoluteCwd(t *testing.T) {
	cwd := t.TempDir()

	runtime, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Cwd: WorkingDirectory(cwd)})

	if err != nil {
		t.Fatalf("construct runtime: %v", err)
	}
	want := filepathDisplay(filepath.Clean(cwd))
	if got := runtime.WorkingDirectory(); got != want {
		t.Fatalf("working directory: got %q, want %q", got, want)
	}
}

func TestNewWithState_failsBeforeEffects_whenInjectedCwdIsMissing(t *testing.T) {
	var stderr bytes.Buffer
	target := filepath.Join(t.TempDir(), "should-not-exist")
	runtime := NewWithState(applets.DefaultRegistry, Streams{Stderr: &stderr}, State{
		Env: NewEnvironment([]string{"PRESERVED=value"}),
	})

	_, resolveErr := runtime.ResolveNemoshPath("relative")
	status := runtime.RunScript(context.Background(), "echo effect >"+filepath.ToSlash(target)+"\n")

	if !errors.Is(resolveErr, ErrStateCwdRequired) {
		t.Fatalf("resolve path: got %v, want %v", resolveErr, ErrStateCwdRequired)
	}
	if status != 1 {
		t.Fatalf("status: got %d, want 1", status)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("effect path: got %v, want not exist", err)
	}
	if value, ok := runtime.LookupEnv("PRESERVED"); !ok || value != "value" {
		t.Fatalf("environment: value=%q present=%t", value, ok)
	}
	if got := runtime.WorkingDirectory(); got != "" {
		t.Fatalf("working directory: got %q, want empty", got)
	}
	if !strings.Contains(stderr.String(), ErrStateCwdRequired.Error()) {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestNewWithState_runInteractiveFailsBeforeEffects_whenInjectedCwdIsMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runtime := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{})
	script, err := ParseScript("echo effect\n")
	if err != nil {
		t.Fatalf("parse script: %v", err)
	}

	result := runtime.RunInteractive(context.Background(), script)

	if result.Status != 1 {
		t.Fatalf("status: got %d, want 1", result.Status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), ErrStateCwdRequired.Error()) {
		t.Fatalf("stderr: %q", stderr.String())
	}
}
