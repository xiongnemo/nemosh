package runtime_test

import "testing"

// `set -n` in a script runs nothing after it, and the shell ends with status 0 -- not the
// rest of the group or function it is in, not an EXIT trap -- while a subshell or a command
// substitution it is set in ends alone. It was refused as needing input still unread, and
// the script went on. busybox-w32 and bash agree on every one of these; in a loop both go
// on for ever, skipping every command of it, where this shell ends.
func TestRuntime_noexecRunsNothingAfterIt(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"echo 1; set -n; echo 2", "1\n"},
		{"echo a; set -n\necho b\necho c", "a\n"},
		{"{ set -n; echo 2; }; echo 3", ""},
		{"f() { set -n; echo 2; }; f; echo 3", ""},
		{"( set -n; echo 2 ); echo 3", "3\n"},
		{"x=$(set -n; echo hi); echo \"[$x]\"", "[]\n"},
		{"trap 'echo bye' EXIT; set -n; echo x", ""},
		{"false; set -n; exit 5", ""},
		{"while true; do set -n; done; echo out", ""},
		{"set -o noexec; echo 2", ""},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, test.want)
			}
		})
	}
}
