package runtime_test

import "testing"

// trap refuses an option it does not know, as both references do, and the script ends there,
// as a special builtin's usage error ends it in busybox-w32. `trap -1 EXIT` armed EXIT with a
// command named -1, which ran as the shell exited. After -- the word is an action again.
func TestTrap_refusesAnUnknownOption(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"trap 'echo trap-exit' EXIT\ntrap -1 EXIT\necho bad\n", "trap-exit\n", 2},
		{"trap -x INT\necho bad\n", "", 2},
		{"trap -- 'echo dd' EXIT\necho ok\n", "ok\ndd\n", 0},
		{"trap 'echo h' EXIT\ntrap - EXIT\necho reset\n", "reset\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
