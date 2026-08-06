package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// env and xargs both launch a COMMAND, and both follow the SUSv3 table BusyBox
// cites: 127 when the command is not found (libbb/executable.c:117-122,
// findutils/xargs.c:385-390).
func TestRuntime_appletsThatLaunchACommandReturn127_whenItIsNotFound(t *testing.T) {
	cases := []struct {
		name   string
		script string
		stderr string
	}{
		{name: "env", script: "env nosuchcommand\n", stderr: "env: nosuchcommand: not found\n"},
		{name: "xargs", script: "echo a | xargs nosuchcommand\n", stderr: "xargs: nosuchcommand: not found\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), testCase.script)

			// Then
			if status != 127 {
				t.Fatalf("expected status 127, got %d", status)
			}
			if got := stderr.String(); got != testCase.stderr {
				t.Fatalf("expected stderr %q, got %q", testCase.stderr, got)
			}
		})
	}
}
