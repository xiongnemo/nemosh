package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func mustParseInteractive(t *testing.T, source string) runtime.Script {
	t.Helper()
	script, err := runtime.ParseScript(source)
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}
	return script
}

func TestRuntime_CloseInteractive_preservesFailureAcrossEmptyPreparedEntries(t *testing.T) {
	for _, source := range []string{"", "# comment only\n"} {
		t.Run(fmt.Sprintf("source %q", source), func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
			rt.RunInteractive(context.Background(), mustParseInteractive(t, "false\n"))

			// When
			rt.RunInteractive(context.Background(), mustParseInteractive(t, source))
			status := rt.CloseInteractive(context.Background())

			// Then
			if status != 1 {
				t.Fatalf("CloseInteractive() = %d, want 1", status)
			}
		})
	}
}

func TestRuntime_CloseInteractive_preservesSuccess(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	rt.RunInteractive(context.Background(), mustParseInteractive(t, "true\n"))

	// When
	status := rt.CloseInteractive(context.Background())

	// Then
	if status != 0 {
		t.Fatalf("CloseInteractive() = %d, want 0", status)
	}
}

func TestRuntime_ReportInteractiveParseError_reportsOnceAndSavesStatusTwo(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	// When
	rt.ReportInteractiveParseError(fmt.Errorf("syntax error: unexpected fi"))
	status := rt.CloseInteractive(context.Background())

	// Then
	if status != 2 {
		t.Fatalf("CloseInteractive() = %d, want 2", status)
	}
	if got, want := stderr.String(), "nemosh: syntax error: unexpected fi\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRuntime_RunInteractive_exitRunsExitTrapOnce(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	rt.RunInteractive(context.Background(), mustParseInteractive(t, "trap 'echo trapped' EXIT\n"))

	// When
	result := rt.RunInteractive(context.Background(), mustParseInteractive(t, "exit 7\n"))
	closeStatus := rt.CloseInteractive(context.Background())

	// Then
	if result != (runtime.InteractiveResult{Status: 7, Exited: true}) {
		t.Fatalf("RunInteractive() = %+v, want status 7 and exited", result)
	}
	if closeStatus != 7 {
		t.Fatalf("CloseInteractive() = %d, want 7", closeStatus)
	}
	if got := stdout.String(); got != "trapped\n" {
		t.Fatalf("trap output = %q, want %q", got, "trapped\n")
	}
}

func TestRuntime_CloseInteractive_runsExitTrapOnce(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	rt.RunInteractive(context.Background(), mustParseInteractive(t, "trap 'echo trapped' EXIT\nfalse\n"))

	// When
	firstStatus := rt.CloseInteractive(context.Background())
	secondStatus := rt.CloseInteractive(context.Background())

	// Then
	if firstStatus != 1 || secondStatus != 1 {
		t.Fatalf("CloseInteractive() statuses = (%d, %d), want (1, 1)", firstStatus, secondStatus)
	}
	if got := stdout.String(); got != "trapped\n" {
		t.Fatalf("trap output = %q, want %q", got, "trapped\n")
	}
}

func TestRuntime_CloseInteractive_failingExitTrapPreservesSavedStatus(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	rt.RunInteractive(context.Background(), mustParseInteractive(t, "trap false EXIT\ntrue\n"))

	// When
	status := rt.CloseInteractive(context.Background())

	// Then
	if status != 0 {
		t.Fatalf("CloseInteractive() = %d, want saved status 0", status)
	}
}

func TestRuntime_RunInteractive_execSuppressesExitTrap(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	rt.RunInteractive(context.Background(), mustParseInteractive(t, "trap 'echo trapped' EXIT\n"))

	// When
	result := rt.RunInteractive(context.Background(), mustParseInteractive(t, "exec true\n"))
	closeStatus := rt.CloseInteractive(context.Background())

	// Then
	if result != (runtime.InteractiveResult{Status: 0, Exited: true}) {
		t.Fatalf("RunInteractive() = %+v, want status 0 and exited", result)
	}
	if closeStatus != 0 {
		t.Fatalf("CloseInteractive() = %d, want 0", closeStatus)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("trap output = %q, want none", got)
	}
}
