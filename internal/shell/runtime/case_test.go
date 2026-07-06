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

func TestRuntime_runsMatchingCaseArm_whenCaseWordMatchesPattern(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"name=two",
		"case $name in",
		"one)",
		"echo one",
		";;",
		"two)",
		"echo two",
		";;",
		"esac",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "two\n" {
		t.Fatalf("expected matching case output %q, got %q", "two\n", got)
	}
}

func TestRuntime_runsFallbackCaseArm_whenNoExactPatternMatches(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"case missing in",
		"one)",
		"echo one",
		";;",
		"*)",
		"echo fallback",
		";;",
		"esac",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "fallback\n" {
		t.Fatalf("expected fallback output %q, got %q", "fallback\n", got)
	}
}

func TestRuntime_runsOnlyFirstCaseArm_whenMultipleArmsMatch(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"case two in",
		"two)",
		"echo first",
		";;",
		"two)",
		"echo second",
		";;",
		"esac",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "first\n" {
		t.Fatalf("expected first match output %q, got %q", "first\n", got)
	}
}

func TestRuntime_returnsSelectedArmStatus_whenCaseCommandFails(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "case x in\nx)\nfalse\n;;\nesac\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
}

func TestRuntime_returnsSyntaxStatus_whenCaseHeaderMissingIn(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "case x\nesac\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if !strings.Contains(stderr.String(), "case:") {
		t.Fatalf("expected case diagnostic, got %q", stderr.String())
	}
}

func TestRuntime_returnsSyntaxStatus_whenCaseMissingEsac(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "case x in\nx)\necho x\n;;\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if !strings.Contains(stderr.String(), "esac") {
		t.Fatalf("expected missing esac diagnostic, got %q", stderr.String())
	}
}

func TestRuntime_returnsSyntaxStatus_whenCaseArmMalformed(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "case x in\nx\necho x\n;;\nesac\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if !strings.Contains(stderr.String(), "case:") {
		t.Fatalf("expected case diagnostic, got %q", stderr.String())
	}
}

func TestRuntime_propagatesExit_whenCaseArmExits(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "case x in\nx)\nexit 7\n;;\nesac\necho after\n")

	// Then
	if status != 7 {
		t.Fatalf("expected status 7, got %d", status)
	}
	if stdout.String() != "" {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}

func TestRuntime_propagatesBreak_whenCaseArmBreaksInsideLoop(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"for item in one two",
		"do",
		"case $item in",
		"one)",
		"echo one",
		"break",
		";;",
		"esac",
		"echo bad",
		"done",
		"echo after",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\nafter\n" {
		t.Fatalf("expected break propagation output %q, got %q", "one\nafter\n", got)
	}
}

func TestRuntime_propagatesContinue_whenCaseArmContinuesInsideLoop(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"for item in one two",
		"do",
		"case $item in",
		"one)",
		"echo one",
		"continue",
		";;",
		"*)",
		"echo $item",
		";;",
		"esac",
		"echo after-$item",
		"done",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "one\ntwo\nafter-two\n" {
		t.Fatalf("expected continue propagation output %q, got %q", "one\ntwo\nafter-two\n", got)
	}
}

func TestRuntime_propagatesReturn_whenCaseArmReturnsFromDotScript(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	scriptPath := writeCaseScript(t, "case x in\nx)\nreturn 5\n;;\nesac\necho unreachable\n")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+" || echo recovered\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "recovered\n" {
		t.Fatalf("expected return propagation output %q, got %q", "recovered\n", got)
	}
}

func TestRuntime_propagatesExec_whenCaseArmExecs(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo bye' EXIT\ncase x in\nx)\nexec echo hi\n;;\nesac\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected exec propagation output %q, got %q", "hi\n", got)
	}
}

func writeCaseScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "library.sh"))
	if err := os.WriteFile(scriptPath, []byte(content), 0o600); err != nil {
		t.Fatalf("expected case fixture write to succeed, got %v", err)
	}
	return scriptPath
}
