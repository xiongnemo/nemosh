package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_nestedCommandSubstitution_balancesInnerSubstitutions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stdout string
	}{
		{name: "single line", source: "echo $(echo $(echo hi))\n", stdout: "hi\n"},
		{name: "multiline", source: "echo $(\necho $(\necho hi\n)\n)\n", stdout: "hi\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), test.source)

			// Then
			if status != 0 || stdout.String() != test.stdout {
				t.Fatalf("RunScript() = status %d, stdout %q, want status 0, stdout %q; stderr = %q", status, stdout.String(), test.stdout, stderr.String())
			}
		})
	}
}

func TestRuntime_commandSubstitution_ignoresQuotedClosingParenthesis(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo $(echo \"a ) b\")\n")

	// Then
	if status != 0 || stdout.String() != "a ) b\n" {
		t.Fatalf("RunScript() = status %d, stdout %q, want status 0, stdout %q; stderr = %q", status, stdout.String(), "a ) b\n", stderr.String())
	}
}

func TestRuntime_compoundBody_bareExitUsesIncomingStatus(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "case", source: "false\ncase selected in\nselected)\nexit\nesac\n"},
		{name: "for", source: "false\nfor item in one\ndo\nexit\ndone\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

			// When
			status := rt.RunScript(context.Background(), test.source)

			// Then
			if status != 1 {
				t.Fatalf("RunScript() status = %d, want 1", status)
			}
		})
	}
}

func TestRuntime_doubleQuotedBackslashNewline_removesContinuation(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo \"one\\\ntwo\"\n")

	// Then
	if status != 0 || stdout.String() != "onetwo\n" {
		t.Fatalf("RunScript() = status %d, stdout %q, want status 0, stdout %q", status, stdout.String(), "onetwo\n")
	}
}

func TestRuntime_syntaxError_doesNotInstallExitTrapFromPrefix(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	source := "trap 'echo trapped' EXIT\necho before\nfi\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 2 {
		t.Fatalf("RunScript() status = %d, want 2", status)
	}
	if got, want := stdout.String(), ""; got != want {
		t.Fatalf("RunScript() stdout = %q, want %q", got, want)
	}
	if count := strings.Count(stderr.String(), "nemosh:"); count != 1 {
		t.Fatalf("RunScript() diagnostic count = %d in %q, want 1", count, stderr.String())
	}
}

func TestRuntime_caseFinalArm_allowsMissingDoubleSemicolon(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	source := "case second in\nfirst)\necho bad\n;;\nsecond)\necho good\nesac\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "good\n" {
		t.Fatalf("RunScript() = status %d, stdout %q, want status 0, stdout %q; stderr = %q", status, stdout.String(), "good\n", stderr.String())
	}
}

func TestRuntime_compoundHeaders_acceptTabsAsShellBlanks(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stdout string
	}{
		{name: "if", source: "if\ttrue\nthen\necho if\nfi\n", stdout: "if\n"},
		{name: "for", source: "for\titem\tin one\ndo\necho $item\ndone\n", stdout: "one\n"},
		{name: "while", source: "while\tfalse\ndo\necho bad\ndone\n", stdout: ""},
		{name: "until", source: "until\ttrue\ndo\necho bad\ndone\n", stdout: ""},
		{name: "case", source: "case\tword\tin\nword)\necho case\nesac\n", stdout: "case\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), test.source)

			// Then
			if status != 0 || stdout.String() != test.stdout {
				t.Fatalf("RunScript() = status %d, stdout %q, want status 0, stdout %q; stderr = %q", status, stdout.String(), test.stdout, stderr.String())
			}
		})
	}
}
