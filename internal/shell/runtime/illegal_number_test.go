package runtime

import "testing"

// The builtins the ash family reads a count or a status with, through one number(), say
// "Illegal number: x" of one that is none, as busybox's ash and dash both do. exit and return
// said bash's "x: numeric argument required", break and continue that or "loop count out of
// range", shift "invalid count: x", and wait "`x': not a pid or valid job spec". The statuses
// were busybox's already.
func TestRuntime_aCountThatIsNoNumberIsAnIllegalNumber(t *testing.T) {
	for script, want := range map[string]string{
		"exit x\n":                          "exit: Illegal number: x\n",
		"f() { return 1x; }\nf\n":           "return: Illegal number: 1x\n",
		"shift -1\n":                        "shift: Illegal number: -1\n",
		"for i in 1; do break 0; done\n":    "break: Illegal number: 0\n",
		"for i in 1; do continue y; done\n": "continue: Illegal number: y\n",
		"wait y\n":                          "wait: Illegal number: y\n",
	} {
		if _, stderr, status := runKill(t, script); stderr != want || status != 2 {
			t.Errorf("%q: stderr %q, status %d; want %q and 2", script, stderr, status, want)
		}
	}
}
