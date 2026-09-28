package runtime_test

import "testing"

// A function is found before a builtin of its name, a special builtin as well, as busybox-w32
// and bash both look a command up; `command` skips the function. The special builtins, and
// the ones that change the flow of control before them, were found first, so a function
// wrapping `exit` or `set` was defined and never called.
func TestRuntime_functionIsFoundBeforeASpecialBuiltin(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"eval() { echo \"eval func $*\"; }; eval 'echo hi'\n", "eval func echo hi\n", 0},
		{"export() { echo \"export func $*\"; }; export X=1\n", "export func X=1\n", 0},
		{"set() { echo 'set func'; }; set -- a\n", "set func\n", 0},
		{"exit() { echo xf; }; exit 3; echo after\n", "xf\nafter\n", 0},
		{"return() { echo rf; }; f() { return 3; echo in; }; f; echo $?\n", "rf\nin\n0\n", 0},
		{"break() { echo bf; }; for i in 1 2; do break; echo $i; done\n", "bf\n1\nbf\n2\n", 0},
		{":() { echo cf; }; :\n", "cf\n", 0},
		{"exit() { echo xf; }; command exit 4; echo after\n", "", 4},
		{"eval() { echo ef; }; command eval 'echo real'\n", "real\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
