package runtime_test

import "testing"

// A `\` that ends a script ends it there, as busybox and bash read one: before the last newline
// it joins nothing and the command ends, and with no newline after it it is a backslash of its
// own. Each was refused as a line waiting for one that never came. From busybox's ash_test
// getopt_nested and bkslash_eof1.
func TestRuntime_backslashEndingTheScript(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"echo start\necho a b | tr a x \\\n", "start\nx b\n"},
		{"echo ok\\", "ok\\\n"},
		{"eval 'echo ok\\'\n", "ok\\\n"},
		{"echo a \\\nb\n", "a b\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
