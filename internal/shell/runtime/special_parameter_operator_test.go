package runtime_test

import "testing"

// `${#...}` is read as busybox's parsesub reads it: the length of a name or a number when a
// letter, a digit or `_` follows the `#`, and of a special parameter when only that one
// character does, so `${##}` is the length of `$#`. Anything else is `$#` itself with an
// operator after it -- `${###}`, `${##2}`, `${#-x}` -- and a special parameter takes an
// operator as a name does: `${?:-x}`, `${-#*f}`. Each was a bad substitution. Busybox and bash
// agree on every row.
func TestRuntime_specialParameterTakesAnOperator(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"set -- $(seq 25); echo ${##} ${###} ${####} ${##2} ${###2} ${#%5} ${#%%5}", "2 25 25 5 5 2 2\n"},
		{"set -- a b c; echo ${##} ${#-x} ${#:-x} ${#:+y} ${#?}", "1 3 3 y 1\n"},
		{"x=abc; echo ${#x} ${##x}", "3 0\n"},
		{"echo \"${?:-x} ${$:+z}\"", "0 z\n"},
		{"set -- a; echo ${#@} ${#*} ${#?}", "1 1 1\n"},
		{"false; echo \"${?#1}\" \"${?%1}x\"", " x\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
