package runtime_test

import "testing"

// A brace group's status is its last command's, which has had its turn at `set -e` and the ERR
// trap already, so the group's own status is not a second turn; and everything inside `!` is
// exempt from both, as busybox-w32 and bash have it. `set -e; { false && true; }` ended the
// script, where the failure is exempt in an && list; `! { false; }` ended it too; and `{
// false; }` fired the ERR trap twice. A subshell still fails in the shell that ran it.
func TestRuntime_errexitAndErrTrapOnGroups(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{"set -e; { false && true; }; echo after", "after\n", 0},
		{"set -e; ! { false; }; echo after", "after\n", 0},
		{"set -e; f() { false; echo in-f; }; ! f; echo after", "in-f\nafter\n", 0},
		{"trap 'echo ERR' ERR; { false; }; echo after", "ERR\nafter\n", 0},
		{"trap 'echo ERR' ERR; { false; false; }; echo after", "ERR\nERR\nafter\n", 0},
		{"trap 'echo ERR' ERR; ( false ); echo after", "ERR\nafter\n", 0},
		{"set -e; { false; }; echo after", "", 1},
		{"set -e; ( false && true ); echo after", "", 1},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as both references answer", stdout, status, test.want, test.status)
			}
		})
	}
}
