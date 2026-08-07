package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runNegationScript(t *testing.T, source string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	return rt.RunScript(context.Background(), source), stdout.String(), stderr.String()
}

func TestRuntime_reportsSuccess_whenBangNegatesAFailingCommand(t *testing.T) {
	// When
	status, _, stderr := runNegationScript(t, "! false\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_reportsFailure_whenBangNegatesASucceedingCommand(t *testing.T) {
	// When
	status, _, stderr := runNegationScript(t, "! true\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_stillRunsTheCommand_whenBangNegatesIt(t *testing.T) {
	// When
	status, stdout, _ := runNegationScript(t, "! echo ran\n")

	// Then
	if status != 1 || stdout != "ran\n" {
		t.Fatalf("status = %d, stdout = %q, want 1 and %q", status, stdout, "ran\n")
	}
}

func TestRuntime_takesTheThenBranch_whenBangNegatesAFailingCondition(t *testing.T) {
	// When
	status, stdout, _ := runNegationScript(t, "if ! false; then echo yes; else echo no; fi\n")

	// Then
	if status != 0 || stdout != "yes\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "yes\n")
	}
}

func TestRuntime_negatesTheWholePipeline_whenBangLeadsIt(t *testing.T) {
	// The status of a pipeline is its last stage's, so this is 0 before the
	// negation and 1 after it.
	// When
	status, _, _ := runNegationScript(t, "! false | true\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
}

func TestRuntime_continuesTheAndOrList_whenBangNegatesItsFirstCommand(t *testing.T) {
	// When
	status, stdout, _ := runNegationScript(t, "! false && echo reached\n")

	// Then
	if status != 0 || stdout != "reached\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "reached\n")
	}
}

func TestRuntime_negatesTheSecondCommand_whenBangFollowsAnAndOperator(t *testing.T) {
	// When
	status, _, stderr := runNegationScript(t, "true && ! false\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_looksUpACommandNamedBang_whenTheBangIsQuoted(t *testing.T) {
	// Quoting removes the reserved-word meaning, so this is an ordinary lookup
	// for a command whose name is one exclamation mark.
	// When
	status, _, stderr := runNegationScript(t, "\"!\" false\n")

	// Then
	if status != 127 {
		t.Fatalf("status = %d, want 127", status)
	}
	if !strings.Contains(stderr, "!") {
		t.Fatalf("stderr = %q, want it to name the command", stderr)
	}
}

func TestRuntime_reportsSyntaxError_whenBangHasNoCommand(t *testing.T) {
	// When
	status, stdout, _ := runNegationScript(t, "!\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
}
