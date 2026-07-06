package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_runsThenBranch_whenIfConditionSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "if true\nthen\necho yes\nelse\necho no\nfi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "yes\n" {
		t.Fatalf("expected then branch output %q, got %q", "yes\n", got)
	}
}

func TestRuntime_runsElseBranch_whenIfConditionFails(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "if false\nthen\necho yes\nelse\necho no\nfi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "no\n" {
		t.Fatalf("expected else branch output %q, got %q", "no\n", got)
	}
}

func TestRuntime_returnsBranchStatus_whenIfBranchCommandFails(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "if true\nthen\nfalse\nfi\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
}

func TestRuntime_runsForBodyForEachWord_whenForLoopHasItems(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "for item in one two\ndo\necho $item\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\n" {
		t.Fatalf("expected loop output %q, got %q", "one\ntwo\n", got)
	}
}

func TestRuntime_returnsLastBodyStatus_whenForLoopCommandFails(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "for item in one\ndo\nfalse\ndone\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
}

func TestRuntime_runsWhileBodyWhileConditionSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin:  bytes.NewBufferString("one\ntwo\n"),
		Stdout: &stdout,
	})

	// When
	status := rt.RunScript(context.Background(), "while read item\ndo\necho $item\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\n" {
		t.Fatalf("expected while output %q, got %q", "one\ntwo\n", got)
	}
}

func TestRuntime_skipsWhileBody_whenConditionFails(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "while false\ndo\necho bad\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
}

func TestRuntime_runsUntilBodyUntilConditionSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\nuntil test $# = 0\ndo\necho $1\nshift\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\n" {
		t.Fatalf("expected until output %q, got %q", "one\ntwo\n", got)
	}
}

func TestRuntime_skipsUntilBody_whenConditionSucceeds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "until true\ndo\necho bad\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
}

func TestRuntime_breaksOutOfUntilLoop_whenBreakRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\nuntil test $# = 0\ndo\necho $1\nbreak\ndone\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\nafter\n" {
		t.Fatalf("expected until break output %q, got %q", "one\nafter\n", got)
	}
}

func TestRuntime_continuesUntilLoop_whenContinueRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- one two\nuntil test $# = 0\ndo\necho $1\nshift\ncontinue\necho bad\ndone\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\nafter\n" {
		t.Fatalf("expected until continue output %q, got %q", "one\ntwo\nafter\n", got)
	}
}

func TestRuntime_breaksOutOfLoop_whenBreakRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "for item in one two\ndo\necho $item\nbreak\ndone\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\nafter\n" {
		t.Fatalf("expected break output %q, got %q", "one\nafter\n", got)
	}
}

func TestRuntime_continuesLoop_whenContinueRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "for item in one two\ndo\necho $item\ncontinue\necho bad\ndone\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\nafter\n" {
		t.Fatalf("expected continue output %q, got %q", "one\ntwo\nafter\n", got)
	}
}
