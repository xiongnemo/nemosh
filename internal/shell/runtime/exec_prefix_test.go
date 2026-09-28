package runtime_test

import "testing"

// A prefix assignment is in the environment of the command exec runs, as it is for any
// command, in busybox-w32 and bash: `pre=x exec printenv pre` says x. It was made in the shell
// alone, unexported, and the command never saw it.
func TestRuntime_execCommandSeesPrefixAssignment(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"pre=x exec printenv pre\n", "x\n"},
		{"a=1 b=2 exec printenv a b\n", "1\n2\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
