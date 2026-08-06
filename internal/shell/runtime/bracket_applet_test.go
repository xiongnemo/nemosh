package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// test_main2 strips argv[0] and demands the last remaining word be a lone "]",
// printing "missing ]" and returning 2 when it is not (coreutils/test.c:897-901,
// NOT_LONE_CHAR at include/libbb.h:1154). With no operands at all the check
// still runs, and argv[0] itself -- "[" -- is what fails it.
func TestRuntime_bracketReturnsStatusTwo_whenTheClosingBracketIsMissing(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{name: "operands but no bracket", script: "[ -n x\n"},
		{name: "trailing word is not a lone bracket", script: "[ -n x ]]\n"},
		{name: "no operands", script: "[\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), testCase.script)

			// Then
			if status != 2 {
				t.Fatalf("expected status 2, got %d", status)
			}
			if got := stderr.String(); got != "[: missing ]\n" {
				t.Fatalf("expected stderr %q, got %q", "[: missing ]\n", got)
			}
		})
	}
}

// A closed bracket keeps the ordinary test statuses: 0 for true, 1 for false,
// and nothing on stderr either way.
func TestRuntime_bracketKeepsTheTestStatuses_whenItIsClosed(t *testing.T) {
	cases := []struct {
		name   string
		script string
		status int
	}{
		{name: "true", script: "[ -n x ]\n", status: 0},
		{name: "false", script: "[ -z x ]\n", status: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), testCase.script)

			// Then
			if status != testCase.status {
				t.Fatalf("expected status %d, got %d", testCase.status, status)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr, got %q", stderr.String())
			}
		})
	}
}
