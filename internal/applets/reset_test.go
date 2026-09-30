package applets_test

import "testing"

// reset writes to a terminal only, as busybox's does, and reads no arguments. There was no reset.
func TestReset_writesToATerminalOnly(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, args := range [][]string{{"reset"}, {"reset", "-Z", "x"}} {
		if stdout, stderr, err := runPermuted(t, view, "", args...); err != nil || stdout != "" || stderr != "" {
			t.Errorf("%q with its output in a buffer: %q, %q, %v; want nothing", args, stdout, stderr, err)
		}
	}
}
