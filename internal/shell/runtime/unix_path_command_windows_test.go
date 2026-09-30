package runtime

import (
	"strings"
	"testing"
)

// A name in /bin, /usr/bin, /sbin or /usr/sbin that is not there is the command of its last
// part, as busybox-w32 runs it: a builtin in the shell, an applet, or a program on PATH, and not
// a function. They were not found. busybox's ash_test glob_and_assign, var5 and var_leaks run
// /bin/echo and /bin/true.
func TestUnixPathCommand_aMissingBinNameIsItsLastPart(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"/bin/echo hi\n", "hi\n"},
		{"/usr/bin/true; echo $?\n", "0\n"},
		{"/usr/sbin/false; echo $?\n", "1\n"},
		{"echo() { builtin echo function; }\n/bin/echo builtin\n", "builtin\n"},
		{"(exec /bin/echo exec)\ncommand /bin/echo command\n", "exec\ncommand\n"},
		{"/bin/export ZZ=1; echo \"$ZZ\"\n", "1\n"},
		{"x=$(/sbin/printf '%s-' a b); echo \"$x\"\n", "a-b-\n"},
		{"/usr/local/bin/echo x 2>/dev/null; echo $?\n", "127\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, stderr, status := runScriptForDevices(t, test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d (stderr %q), want %q/0, as busybox-w32 answers", stdout, status, stderr, test.want)
			}
		})
	}
	stdout, stderr, status := runScriptForDevices(t, "/bin/nemosh-no-such-command\necho $?\n")
	if stdout != "127\n" || !strings.HasPrefix(stderr, "nemosh-no-such-command: not found") || status != 0 {
		t.Errorf("got %q/%d, stderr %q; want 127 and the last part named, as busybox-w32 names it", stdout, status, stderr)
	}
}
