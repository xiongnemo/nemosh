package runtime_test

import "testing"

// A backquoted command that begins with `(` is a subshell, as busybox-w32 and bash run it.
// Backquotes are rewritten to `$(...)`, and `(echo a)` became `$((echo a))`, an arithmetic
// expansion: `(cd dir && pwd)` in backquotes was an arithmetic syntax error.
func TestRuntime_backquotedSubshell(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"x=`(echo a)`; echo \"[$x]\"\n", "[a]\n"},
		{"x=`((echo a) || (echo b))`; echo \"[$x]\"\n", "[a]\n"},
		{"x=`(echo a); echo b`; echo \"[$x]\"\n", "[a\nb]\n"},
		{"echo `(echo a) | cat`\n", "a\n"},
		{"x=`((echo a) ||\n  (echo b)) 2>/dev/null`\necho \"[$x]\"\n", "[a]\n"},
		{"echo `echo $((1+2))`\n", "3\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
