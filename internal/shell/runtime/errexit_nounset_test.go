package runtime_test

import (
	"strings"
	"testing"
)

func TestRuntime_stopsTheScript_whenErrExitIsOnAndACommandFails(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\nfalse\necho STILL\n")

	// Then
	if status != 1 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 1 and no output", status, stdout)
	}
}

func TestRuntime_keepsGoing_whenErrExitIsOnAndTheCommandSucceeds(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\ntrue\necho reached\n")

	// Then
	if status != 0 || stdout != "reached\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "reached\n")
	}
}

func TestRuntime_doesNotExit_whenAFailingCommandIsAnIfCondition(t *testing.T) {
	// POSIX 2.9.1 exempts the condition of an if, so this is the whole reason
	// `set -e` is usable at all.
	// When
	status, stdout, _ := runSetScript(t, "set -e\nif false; then echo yes; else echo no; fi\necho after\n")

	// Then
	if status != 0 || stdout != "no\nafter\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "no\nafter\n")
	}
}

func TestRuntime_doesNotExit_whenAFailingCommandIsAWhileCondition(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\nwhile false; do echo never; done\necho after\n")

	// Then
	if status != 0 || stdout != "after\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "after\n")
	}
}

func TestRuntime_doesNotExit_whenAFailingCommandIsNotLastInAnAndOrList(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\nfalse || echo recovered\necho after\n")

	// Then
	if status != 0 || stdout != "recovered\nafter\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "recovered\nafter\n")
	}
}

func TestRuntime_doesNotExit_whenAFailingCommandIsNegated(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\n! false\necho after\n")

	// Then
	if status != 0 || stdout != "after\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "after\n")
	}
}

func TestRuntime_exitsFromInsideAFunction_whenErrExitIsOnAndACommandFails(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\nf() { false; echo body; }\nf\necho after\n")

	// Then
	if status != 1 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 1 and no output", status, stdout)
	}
}

func TestRuntime_stopsWithTheFailingStatus_whenErrExitIsOn(t *testing.T) {
	// When
	status, _, _ := runSetScript(t, "set -e\nexit 7\n")

	// Then
	if status != 7 {
		t.Fatalf("status = %d, want 7", status)
	}
}

func TestRuntime_reportsTheUnsetName_whenNoUnsetIsOn(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -u\necho [$nope]\necho STILL\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
	if !strings.Contains(stderr, "nope") || !strings.Contains(stderr, "parameter not set") {
		t.Fatalf("stderr = %q, want it to name nope as an unset parameter", stderr)
	}
}

func TestRuntime_reportsABracedUnsetName_whenNoUnsetIsOn(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "set -u\necho [${nope}]\n")

	// Then
	if status != 2 || !strings.Contains(stderr, "nope") {
		t.Fatalf("status = %d, stderr = %q, want 2 and a diagnostic naming nope", status, stderr)
	}
}

func TestRuntime_acceptsAnEmptyButSetName_whenNoUnsetIsOn(t *testing.T) {
	// -u is about being unset, not about being empty.
	// When
	status, stdout, _ := runSetScript(t, "set -u\nempty=\necho [$empty]\n")

	// Then
	if status != 0 || stdout != "[]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[]\n")
	}
}

func TestRuntime_acceptsADefaultedName_whenNoUnsetIsOn(t *testing.T) {
	// A default supplies the value, so nothing is unset by the time it is used.
	// When
	status, stdout, _ := runSetScript(t, "set -u\necho [${nope:-fallback}]\n")

	// Then
	if status != 0 || stdout != "[fallback]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[fallback]\n")
	}
}

func TestRuntime_reportsAnUnsetPositional_whenNoUnsetIsOn(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "set -u\nset a\necho [$2]\n")

	// Then
	if status != 2 || !strings.Contains(stderr, "parameter not set") {
		t.Fatalf("status = %d, stderr = %q, want 2 and an unset-parameter diagnostic", status, stderr)
	}
}
