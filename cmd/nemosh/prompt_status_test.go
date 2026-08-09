package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func promptWithStatus(t *testing.T, ps1 string, status int) string {
	t.Helper()
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	rt.SetVariable("PS1", ps1)
	return interactivePromptWithStatus(context.Background(), rt, false, status)
}

// A prompt that reports the last exit code is the common reason to put a
// substitution in PS1 at all -- every framework's prompt does it, and the one on
// this machine writes `C:1` in red when a command fails.
//
// The direct form worked; the substituted one did not. A command substitution
// runs its own script, and that script saw its own $? rather than the status the
// prompt was drawn for, so `$(prompt_info $?)` always received zero.
func TestPrompt_carriesTheStatusIntoASubstitution(t *testing.T) {
	for _, test := range []struct {
		name string
		ps1  string
		want string
	}{
		{name: "directly", ps1: `[$?]`, want: "[7]"},
		{name: "through a substitution", ps1: `[$(echo $?)]`, want: "[7]"},
		{name: "through a nested one", ps1: `[$(echo $(echo $?))]`, want: "[7]"},
		{name: "as an argument to a command", ps1: `[$(printf %s $?)]`, want: "[7]"},
		{name: "backquoted", ps1: "[`echo $?`]", want: "[7]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := promptWithStatus(t, test.ps1, 7); got != test.want {
				t.Fatalf("prompt = %q, want %q", got, test.want)
			}
		})
	}
}

// Zero is carried too, so a prompt that only decorates a failure stays quiet on
// success rather than always decorating.
func TestPrompt_carriesZeroIntoASubstitution(t *testing.T) {
	if got := promptWithStatus(t, `[$(echo $?)]`, 0); got != "[0]" {
		t.Fatalf("prompt = %q, want %q", got, "[0]")
	}
}

// The shape a real startup file uses: a function that reads $1 and decorates
// only on failure.
func TestPrompt_carriesTheStatusToAFunction(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if status := rt.RunScript(context.Background(), "prompt_info() {\nif [ \"$1\" != 0 ]; then\necho \"C:$1\"\nfi\n}\n"); status != 0 {
		t.Fatalf("defining the function failed with %d", status)
	}
	rt.SetVariable("PS1", `$(prompt_info $?)> `)

	// When
	failed := interactivePromptWithStatus(context.Background(), rt, false, 1)
	succeeded := interactivePromptWithStatus(context.Background(), rt, false, 0)

	// Then
	if !strings.HasPrefix(failed, "C:1") {
		t.Fatalf("after a failure the prompt = %q, want it to start with C:1", failed)
	}
	if strings.Contains(succeeded, "C:") {
		t.Fatalf("after a success the prompt = %q, want no status decoration", succeeded)
	}
}
