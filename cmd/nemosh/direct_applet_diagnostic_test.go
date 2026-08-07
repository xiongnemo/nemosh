package main

import (
	"strings"
	"testing"
)

// A direct invocation and the same command inside the shell are the same
// applet, so they have to fail the same way. Direct dispatch used to drop the
// applet-name prefix, and to print nothing at all when the failure carried its
// own status -- so `nemosh env python3` exited 127 in silence while
// `nemosh -c 'env python3'` said `env: python3: not found`.
func TestRun_directAppletDiagnosticsMatchTheShell(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		direct   []string
		script   string
		status   int
		fragment string
	}{
		{
			name:     "an unreadable operand",
			direct:   []string{"cat", "nosuchfile.txt"},
			script:   "cat nosuchfile.txt",
			status:   1,
			fragment: "cat: can't open 'nosuchfile.txt'",
		},
		{
			name:     "a command env cannot run",
			direct:   []string{"env", "definitely-not-a-program"},
			script:   "env definitely-not-a-program",
			status:   127,
			fragment: "env: definitely-not-a-program: not found",
		},
		{
			name:     "grep on a missing file",
			direct:   []string{"grep", "pattern", "nosuchfile.txt"},
			script:   "grep pattern nosuchfile.txt",
			status:   2,
			fragment: "grep: nosuchfile.txt",
		},
		{
			name:     "rmdir on a missing directory",
			direct:   []string{"rmdir", "nosuchdir"},
			script:   "rmdir nosuchdir",
			status:   1,
			fragment: "rmdir: 'nosuchdir'",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			direct := runArgs(t, "", append([]string{"nemosh"}, testCase.direct...)...)
			inShell := runArgs(t, "", "nemosh", "-c", testCase.script)

			// Then
			if direct.status != testCase.status {
				t.Fatalf("direct status = %d, want %d (stderr %q)", direct.status, testCase.status, direct.stderr)
			}
			if !strings.Contains(direct.stderr, testCase.fragment) {
				t.Fatalf("direct stderr = %q, want it to contain %q", direct.stderr, testCase.fragment)
			}
			if direct.stderr != inShell.stderr {
				t.Fatalf("direct stderr = %q, in-shell stderr = %q, want them identical", direct.stderr, inShell.stderr)
			}
			if direct.status != inShell.status {
				t.Fatalf("direct status = %d, in-shell status = %d, want them identical", direct.status, inShell.status)
			}
		})
	}
}

func TestRun_directAppletStaysSilent_whenTheFailureCarriesNoDiagnostic(t *testing.T) {
	// `false` fails without anything to say, and must not grow a message here.
	// When
	got := runArgs(t, "", "nemosh", "false")

	// Then
	if got.status != 1 {
		t.Fatalf("status = %d, want 1", got.status)
	}
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want nothing", got.stderr)
	}
}

func TestRun_directAppletSucceedsQuietly(t *testing.T) {
	// When
	got := runArgs(t, "", "nemosh", "true")

	// Then
	if got.err != nil || got.stderr != "" {
		t.Fatalf("err = %v, stderr = %q, want neither", got.err, got.stderr)
	}
}
