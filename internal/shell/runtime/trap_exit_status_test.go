package runtime_test

import "testing"

// An `exit` given no status in a trap's action ends with the status the trap was entered with,
// as POSIX has it and busybox and bash do -- the last command before the trap, not the last in
// it: `trap 'echo bye; exit' EXIT; false` ends with 1. It ended with the echo's 0, so a
// failing script with a cleanup trap reported success. busybox's ash_test exitcode_trap1.
func TestRuntime_exitInATrapKeepsTheStatusItCameWith(t *testing.T) {
	for _, test := range []struct {
		script, want string
		status       int
	}{
		{"trap 'echo Trapped; exit' EXIT; false\n", "Trapped\n", 1},
		{"(trap 'echo Trapped; exit' EXIT; (exit 1)); echo One:$?\n", "Trapped\nOne:1\n", 0},
		{"trap 'echo T; exit 5' EXIT; false\n", "T\n", 5},
		{"f() { exit; }; trap 'echo in; f' EXIT; (exit 7)\n", "in\n", 7},
		{"trap 'false; exit' EXIT; true\n", "", 0},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as busybox and bash answer", stdout, status, test.want, test.status)
			}
		})
	}
}
