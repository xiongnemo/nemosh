package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runCaseScript(t *testing.T, source string) (int, string) {
	t.Helper()
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	return rt.RunScript(context.Background(), source), stdout.String()
}

func TestRuntime_selectsTheMatchingArm_whenTheWholeCaseIsOnOneLine(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case b in a) echo A;; b) echo B;; esac\n")

	// Then
	if status != 0 || stdout != "B\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "B\n")
	}
}

func TestRuntime_selectsTheDefaultArm_whenAOneLineCaseMatchesNothingElse(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case zz in a) echo A;; *) echo other;; esac\n")

	// Then
	if status != 0 || stdout != "other\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "other\n")
	}
}

func TestRuntime_matchesAnAlternation_whenAOneLinePatternListsTwoWords(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case b in a|b) echo AB;; esac\n")

	// Then
	if status != 0 || stdout != "AB\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "AB\n")
	}
}

func TestRuntime_runsEveryCommandInAnArm_whenAOneLineArmHasSeveral(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in a) echo one; echo two;; esac\n")

	// Then
	if status != 0 || stdout != "one\ntwo\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "one\ntwo\n")
	}
}

func TestRuntime_runsANestedCompound_whenItSharesTheArmLine(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in a) if true; then echo nested; fi;; esac\n")

	// Then
	if status != 0 || stdout != "nested\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "nested\n")
	}
}

func TestRuntime_keepsAQuotedParenthesisAsData_whenItAppearsInAnArmBody(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in a) echo ')';; esac\n")

	// Then
	if status != 0 || stdout != ")\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, ")\n")
	}
}

func TestRuntime_acceptsAnArmTerminatorOnTheBodyLine_whenTheCaseSpansLines(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in\na) echo A;;\nb) echo B;;\nesac\n")

	// Then
	if status != 0 || stdout != "A\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "A\n")
	}
}

func TestRuntime_selectsTheInnerArm_whenAOneLineCaseNestsInsideAnother(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in a) case y in y) echo inner;; esac;; esac\n")

	// Then
	if status != 0 || stdout != "inner\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "inner\n")
	}
}

func TestRuntime_reportsSyntaxError_whenAOneLineCaseArmHasNoPattern(t *testing.T) {
	// When
	status, stdout := runCaseScript(t, "case a in echo A;; esac\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
}
