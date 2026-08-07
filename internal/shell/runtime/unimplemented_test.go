package runtime_test

import (
	"strings"
	"testing"
)

// A builtin this shell knows and does not implement used to fall through to
// command lookup and come back as `fg: not found` with 127 -- the same words a
// missing external program gets, so a reader could not tell "this shell does
// not have it" from "install it".
func TestRuntime_refusesABuiltinItDoesNotImplement(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		fragments []string
	}{
		{
			name:      "hash",
			script:    "hash\n",
			fragments: []string{"hash: not implemented", "not cached", "busybox-w32 does implement it"},
		},
		{
			name:      "ulimit",
			script:    "ulimit -n\n",
			fragments: []string{"ulimit: not implemented", "no getrlimit", "busybox-w32 does not implement it either", "returns 1 with no message"},
		},
		{
			name:      "fg",
			script:    "fg\n",
			fragments: []string{"fg: not implemented", "terminal process group", "compiled out"},
		},
		{
			name:      "bg",
			script:    "bg\n",
			fragments: []string{"bg: not implemented", "terminal process group", "compiled out"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			// 126, not 127: SUSv3 keeps 127 for a command that could not be
			// found and 126 for one that was found and could not be run.
			if status != 126 {
				t.Fatalf("status = %d, want 126 (stderr %q)", status, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want nothing", stdout)
			}
			for _, fragment := range test.fragments {
				if !strings.Contains(stderr, fragment) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, fragment)
				}
			}
		})
	}
}

func TestRuntime_describesAnUnimplementedBuiltinToType(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "type ulimit\n")

	// Then
	if status != 0 || !strings.Contains(stdout, "does not implement") {
		t.Fatalf("status = %d, stdout = %q, want type to say it is a builtin this shell does not implement", status, stdout)
	}
}

func TestRuntime_answersNoFromCommandV_forAnUnimplementedBuiltin(t *testing.T) {
	// `command -v` is what a script uses to decide whether it can rely on
	// something, so it has to say no.
	// When
	status, stdout, _ := runSetScript(t, "command -v ulimit\n")

	// Then
	if status != 1 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 1 and no output", status, stdout)
	}
}

func TestRuntime_refusesASetOptionThatWouldBeInert(t *testing.T) {
	// Storing an option nothing reads and reporting it through `$-` is the same
	// shape of lie as an applet swallowing a flag.
	// When
	status, _, stderr := runSetScript(t, "set -b\n")

	// Then
	if status != 2 || !strings.Contains(stderr, "not implemented") {
		t.Fatalf("status = %d, stderr = %q, want 2 and a not-implemented diagnostic", status, stderr)
	}
}

func TestRuntime_allowsTurningAnInertOptionOff(t *testing.T) {
	// `set +b` asks for the state it is already in, so there is nothing to
	// refuse.
	// When
	status, _, stderr := runSetScript(t, "set +b\n")

	// Then
	if status != 0 || stderr != "" {
		t.Fatalf("status = %d, stderr = %q, want 0 and nothing", status, stderr)
	}
}
