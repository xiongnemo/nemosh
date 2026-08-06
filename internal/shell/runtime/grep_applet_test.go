package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// grep_main raises xfunc_error_retval to 2 before parsing so that 1 stays
// reserved for "no match" (findutils/grep.c:718). Every other failure exits 2.
func TestRuntime_grepReturnsStatusTwo_whenTheInvocationIsWrong(t *testing.T) {
	cases := []struct {
		name   string
		script string
		stderr string
	}{
		{name: "no pattern", script: "grep\n", stderr: "grep: missing pattern\n"},
		{name: "unsupported option", script: "grep -z one\n", stderr: "grep: unsupported grep option: -z\n"},
		{name: "unreadable operand", script: "grep one nope.txt\n", stderr: "grep: nope.txt: No such file or directory\n"},
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
			if got := stderr.String(); got != testCase.stderr {
				t.Fatalf("expected stderr %q, got %q", testCase.stderr, got)
			}
		})
	}
}

func TestRuntime_grepReturnsStatusOne_whenNothingMatches(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: bytes.NewBufferString("one\n"), Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "grep absent\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}
