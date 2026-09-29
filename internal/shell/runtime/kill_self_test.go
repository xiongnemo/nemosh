package runtime_test

import "testing"

// `kill -SIG $$` is the shell's signal to itself, taken as bash takes it: a trap for it runs
// once the kill has finished and the script goes on, one an empty trap ignores changes nothing,
// and one nothing catches ends the script with 128+n, its EXIT trap still running. It ended
// the process on the spot, with no trap run and an exit code a parent read as 0.
func TestKill_theShellsOwnPidRaisesTheSignalInTheShell(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"trap 'echo trapped' TERM\nkill -TERM $$\necho after=$?\n", "trapped\nafter=0\n", 0},
		{"trap 'echo trapped-int' INT\nkill -INT $$\necho after\n", "trapped-int\nafter\n", 0},
		{"trap '' HUP\nkill -HUP $$\necho ignored\n", "ignored\n", 0},
		{"trap 'echo bye' EXIT\nkill $$\necho not reached\n", "bye\n", 143},
		{"kill -0 $$ && echo alive\n", "alive\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as bash answers", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
