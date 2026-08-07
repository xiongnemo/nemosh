package runtime_test

import (
	"strings"
	"testing"
)

func TestRuntime_runsTheHandler_whenAnExitTrapIsArmed(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "trap 'echo bye' EXIT\necho body\n")

	// Then
	if status != 0 || stdout != "body\nbye\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "body\nbye\n")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic", stderr)
	}
}

func TestRuntime_dropsTheHandler_whenTheActionIsALoneDash(t *testing.T) {
	// busybox ash maps a lone `-` to a nil action (LONE_DASH, shell/ash.c), so
	// this resets EXIT to its default of doing nothing.
	// When
	status, stdout, stderr := runSetScript(t, "trap 'echo bye' EXIT\ntrap - EXIT\necho body\n")

	// Then
	if status != 0 || stdout != "body\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "body\n")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no diagnostic, got a stray lookup", stderr)
	}
}

func TestRuntime_dropsTheHandler_whenOnlyTheConditionIsGiven(t *testing.T) {
	// `trap EXIT` is the same as `trap - EXIT`: with one operand there is no
	// action word to read, which is the rule busybox spells out at trapcmd.
	// When
	status, stdout, _ := runSetScript(t, "trap 'echo bye' EXIT\ntrap EXIT\necho body\n")

	// Then
	if status != 0 || stdout != "body\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "body\n")
	}
}

func TestRuntime_reportsNothing_whenTrapListsAnEmptyTable(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "trap\n")

	// Then
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want 0 and nothing", status, stdout, stderr)
	}
}

func TestRuntime_listsTheArmedHandlers_whenTrapHasNoArguments(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "trap 'echo bye' EXIT\ntrap\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if !strings.Contains(stdout, "trap -- 'echo bye' EXIT") {
		t.Fatalf("stdout = %q, want a reusable trap listing", stdout)
	}
}

func TestRuntime_quotesTheListedAction_whenItContainsASingleQuote(t *testing.T) {
	// The listing has to be readable back in, so an embedded quote closes,
	// escapes, and reopens.
	// When
	status, stdout, _ := runSetScript(t, "trap \"echo it's\" INT\ntrap\n")

	// Then
	if status != 0 || !strings.Contains(stdout, `trap -- 'echo it'\''s' INT`) {
		t.Fatalf("status = %d, stdout = %q, want the action requoted", status, stdout)
	}
}

func TestRuntime_armsEveryCondition_whenTrapNamesSeveral(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "trap 'echo handler' EXIT INT\ntrap\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if !strings.Contains(stdout, "EXIT") || !strings.Contains(stdout, "INT") {
		t.Fatalf("stdout = %q, want both conditions listed", stdout)
	}
}

func TestRuntime_acceptsTheZeroSynonym_whenTrapNamesTheExitCondition(t *testing.T) {
	// POSIX lets 0 stand for EXIT.
	// When
	status, stdout, _ := runSetScript(t, "trap 'echo bye' 0\necho body\n")

	// Then
	if status != 0 || stdout != "body\nbye\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "body\nbye\n")
	}
}

func TestRuntime_reportsAnInvalidCondition_whenTheNameIsNotASignal(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "trap 'echo x' BOGUS\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr, "BOGUS") || !strings.Contains(stderr, "invalid signal specification") {
		t.Fatalf("stderr = %q, want bash's invalid-signal wording", stderr)
	}
}

func TestRuntime_saysSoPlainly_whenTheSignalIsRealButUnsupported(t *testing.T) {
	// TERM is a real signal and a typo is not what happened, so the diagnostic
	// must not claim the name is invalid.
	// When
	status, _, stderr := runSetScript(t, "trap 'echo x' TERM\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr, "TERM") || strings.Contains(stderr, "invalid signal specification") {
		t.Fatalf("stderr = %q, want TERM reported as unsupported rather than invalid", stderr)
	}
}

func TestRuntime_ignoresTheCondition_whenTheActionIsEmpty(t *testing.T) {
	// An empty action means ignore, which is different from having no handler.
	// When
	status, stdout, _ := runSetScript(t, "trap '' EXIT\necho body\n")

	// Then
	if status != 0 || stdout != "body\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "body\n")
	}
}
