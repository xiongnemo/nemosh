package runtime_test

import "testing"

// A lone `+` among set's options is an ignored flag, as busybox reads it: `set +` changed the
// positional parameters to one `+`, and `set -x + y z` made `+` the first of them. And a lone
// `-` ends the options, turning off -x, and leaves the positional parameters alone when no
// operand follows it; they were cleared. The answers are busybox-w32's.
func TestRuntime_setLoneFlags(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"set a b; set +; echo \"[$*] $#\"", "[a b] 2\n"},
		{"set + x; echo \"[$*]\"", "[x]\n"},
		{"set a b; set -; echo \"[$*] $#\"", "[a b] 2\n"},
		{"set - +; echo \"[$*]\"; set + -; echo \"[$*]\"", "[+]\n[+]\n"},
		{"set -- +; echo \"[$*]\"", "[+]\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, test.want)
			}
		})
	}
}
