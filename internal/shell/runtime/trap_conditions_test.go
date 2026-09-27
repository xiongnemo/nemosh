package runtime_test

import "testing"

// trap reads its conditions as both references read them, and an EXIT trap's `exit` is how
// the shell ends:
//   - a signal's name in any case, with SIG in front or not: `trap - int` left INT armed;
//   - an unsigned number first means every operand is a condition to reset, as POSIX has
//     it: `trap 0 2` armed INT with a command named 0;
//   - `exit N` in an EXIT trap ends the shell with N -- a script's, a subshell's, a command
//     substitution's -- where it ended with the status it had before the trap ran.
//
// The answers are busybox-w32's, and bash agrees on each.
func TestRuntime_trapConditionsAndExitTrapStatus(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{"trap 'echo x' int; trap; trap - Int; trap; echo end", "trap -- 'echo x' INT\nend\n", 0},
		{"trap 'echo x' sigint; trap 'echo e' exit; trap", "trap -- 'echo e' EXIT\ntrap -- 'echo x' INT\ne\n", 0},
		{"trap 'echo e' EXIT; trap 'echo i' INT; trap 0 2; trap; echo end", "end\n", 0},
		{"trap 'echo i' INT; trap 'echo e' EXIT; trap 2 EXIT; trap; echo end", "end\n", 0},
		{"trap 'exit 42' EXIT; echo body", "body\n", 42},
		{"trap 'echo in; exit 7' EXIT; exit 3", "in\n", 7},
		{"trap false EXIT; exit 3", "", 3},
		{"( trap 'exit 5' EXIT; true ); echo st=$?", "st=5\n", 0},
		{"x=$(trap 'exit 6' EXIT; echo body); echo \"[$x] st=$?\"", "[body] st=6\n", 0},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as busybox-w32 answers", stdout, status, test.want, test.status)
			}
		})
	}
}
