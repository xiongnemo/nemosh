package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runPrefixScript(t *testing.T, source string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	return rt.RunScript(context.Background(), source), stdout.String(), stderr.String()
}

func TestRuntime_exits_whenAnAssignmentPrefixesExit(t *testing.T) {
	// When
	status, stdout, stderr := runPrefixScript(t, "V=1 exit 3\necho NOT-EXITED\n")

	// Then
	if status != 3 {
		t.Fatalf("status = %d, want 3", status)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing after the exit", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_leavesTheLoop_whenAnAssignmentPrefixesBreak(t *testing.T) {
	// When
	status, stdout, stderr := runPrefixScript(t, "for i in 1 2 3; do V=x break; done\necho after\n")

	// Then
	if status != 0 || stdout != "after\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "after\n")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_keepsTheAssignment_whenItPrefixesASpecialBuiltin(t *testing.T) {
	// A special builtin's leading assignments persist after it completes
	// (POSIX 2.9.1), and `break` is one of them.
	// When
	status, stdout, _ := runPrefixScript(t, "for i in 1; do V=kept break; done\necho [$V]\n")

	// Then
	if status != 0 || stdout != "[kept]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[kept]\n")
	}
}

func TestRuntime_startsTheNextIteration_whenAnAssignmentPrefixesContinue(t *testing.T) {
	// When
	status, stdout, _ := runPrefixScript(t, "for i in 1 2; do V=x continue; echo body; done\necho after\n")

	// Then
	if status != 0 || stdout != "after\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "after\n")
	}
}

func TestRuntime_returnsFromTheFunction_whenAnAssignmentPrefixesReturn(t *testing.T) {
	// When
	status, stdout, _ := runPrefixScript(t, "f() { V=1 return 3; echo body; }\nf\necho [$?]\n")

	// Then
	if status != 0 || stdout != "[3]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[3]\n")
	}
}

func TestRuntime_doesNotLoopForever_whenAnAssignmentPrefixesBreakInAWhile(t *testing.T) {
	// The regression this pins is unbounded: dispatching on the assignment
	// instead of the command turned `V=x break` into a failed lookup, so the
	// loop never ended.
	// When
	status, stdout, _ := runPrefixScript(t, "while true; do V=x break; done\necho done\n")

	// Then
	if status != 0 || stdout != "done\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "done\n")
	}
}
