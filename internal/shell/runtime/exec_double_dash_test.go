package runtime_test

import "testing"

// `--` ends exec's options, as busybox-w32 and bash read it: with nothing after it exec
// applies its redirections to the shell, and otherwise it runs what follows. It was run as a
// command called --, not found.
func TestRuntime_execTakesDoubleDash(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"exec -- 3>&1\necho stdout 1>&3\n", "stdout\n"},
		{"exec -- echo hi\necho not reached\n", "hi\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
