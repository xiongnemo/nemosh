package runtime_test

import "testing"

// eval takes an array literal as its operand, as bash reads it (busybox-w32 has no arrays):
// `eval a=( ${list[@]} )` runs the assignment. It was refused as a literal after a command
// name, and the script never began. After any other command it is still refused.
func TestRuntime_evalTakesAnArrayLiteral(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"eval a=(1 2); echo ${#a[@]}", "2\n"},
		{"n=(x y); eval b=( ${n[@]} ); echo ${#b[@]} ${b[1]}", "2 y\n"},
		{"eval c+=(3) c+=(4); echo ${c[@]}", "3 4\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
	for _, script := range []string{"true a=(1 2)\necho after", "command a=(1 2)\necho after"} {
		if stdout, status := runScriptCapturing(script + "\n"); status != 2 || stdout != "" {
			t.Errorf("%q: got %q/%d, want a syntax error and nothing run", script, stdout, status)
		}
	}
}
