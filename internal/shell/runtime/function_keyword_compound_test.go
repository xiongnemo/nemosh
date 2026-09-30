package runtime_test

import "testing"

// With the function keyword, a body that is a bare compound needs no parentheses before it, as
// busybox and bash both take it: `function f for i in 1 2 3; do ...; done`. It was "unexpected
// do". busybox's ash_test func_bash1.
func TestRuntime_aKeywordFunctionMayHaveABareCompoundBody(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"function f for i in 1 2 3; do\n\techo $i\ndone\nf\n", "1\n2\n3\n"},
		{"function f if true; then echo I; fi\nf\n", "I\n"},
		{"function f while false; do :; done\nf; echo w=$?\n", "w=0\n"},
		{"function f case x in x) echo C;; esac\nf\n", "C\n"},
		{"function f { echo brace; }\nf\n", "brace\n"},
		{"function g () for i in 1; do echo g$i; done\ng\n", "g1\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
