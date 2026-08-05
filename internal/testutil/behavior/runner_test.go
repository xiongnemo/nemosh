package behavior_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

func TestRunner_executesScriptInSandbox_withInputs(t *testing.T) {
	// Given
	parentCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	caseData := behavior.Case{Script: "test", Stdin: "input", CWD: "work", Env: map[string]string{"EMPTY": ""}, Files: map[string]string{"work/file.txt": "fixture"}}
	executor := func(_ context.Context, request behavior.ScriptRequest) (behavior.ProcessResult, error) {
		data, readErr := os.ReadFile(filepath.Join(request.Dir, "file.txt"))
		if readErr != nil {
			return behavior.ProcessResult{}, readErr
		}
		if request.Stdin != "input" || string(data) != "fixture" {
			return behavior.ProcessResult{}, errors.New("sandbox inputs differ")
		}
		return behavior.ProcessResult{Status: 7, Stdout: "out", Stderr: "err"}, nil
	}
	runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, executor)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.HarnessError != nil || result.Status != 7 || result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("unexpected result: %#v", result)
	}
	afterCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if afterCWD != parentCWD {
		t.Fatalf("parent cwd changed from %q to %q", parentCWD, afterCWD)
	}
}

func TestRunner_skipsBeforeSetup_whenPlatformDoesNotMatch(t *testing.T) {
	// Given
	caseData := behavior.Case{Script: "test", Platforms: []string{"unsupported"}, Files: map[string]string{"../escape": "bad"}}
	runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, func(context.Context, behavior.ScriptRequest) (behavior.ProcessResult, error) {
		t.Fatal("executor must not run")
		return behavior.ProcessResult{}, nil
	})

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.SkipReason == "" || result.HarnessError != nil {
		t.Fatalf("expected explicit skip, got %#v", result)
	}
}

func TestRunner_reportsHarnessErrorSeparately(t *testing.T) {
	// Given
	wantErr := errors.New("executor failed")
	runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, func(context.Context, behavior.ScriptRequest) (behavior.ProcessResult, error) {
		return behavior.ProcessResult{}, wantErr
	})

	// When
	result := runner.Run(context.Background(), behavior.Case{Script: "test"})

	// Then
	if !errors.Is(result.HarnessError, wantErr) {
		t.Fatalf("expected harness error %v, got %v", wantErr, result.HarnessError)
	}
}

func TestRunner_executesCommandCase_whenAppletExists(t *testing.T) {
	// Given
	caseData := behavior.Case{
		Command: []string{"echo", "hello", "world"},
		Expect:  behavior.Expect{Status: 0, Stdout: "hello world\n", Stderr: ""},
	}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != caseData.Expect.Status {
		t.Fatalf("expected status %d, got %d", caseData.Expect.Status, result.Status)
	}
	if result.Stdout != caseData.Expect.Stdout {
		t.Fatalf("expected stdout %q, got %q", caseData.Expect.Stdout, result.Stdout)
	}
	if result.Stderr != caseData.Expect.Stderr {
		t.Fatalf("expected stderr %q, got %q", caseData.Expect.Stderr, result.Stderr)
	}
}

func TestRunner_returnsCommandNotFound_whenAppletMissing(t *testing.T) {
	// Given
	caseData := behavior.Case{Command: []string{"missing-applet"}}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != 127 {
		t.Fatalf("expected status 127, got %d", result.Status)
	}
	if result.Stderr != "missing-applet: not found\n" {
		t.Fatalf("expected not found stderr, got %q", result.Stderr)
	}
}

func TestRunner_returnsAppletStatus_whenAppletReportsStatusCode(t *testing.T) {
	// Given
	caseData := behavior.Case{Command: []string{"sort", "-z"}}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != 2 {
		t.Fatalf("expected status 2, got %d", result.Status)
	}
	if result.Stdout != "" {
		t.Fatalf("expected empty stdout, got %q", result.Stdout)
	}
	if result.Stderr != "sort: invalid option -- z\n" {
		t.Fatalf("expected invalid option stderr, got %q", result.Stderr)
	}
}
