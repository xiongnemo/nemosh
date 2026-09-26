package runtime_test

import "testing"

// A function's body can be any compound command, as POSIX 2.9.5 has it and both references
// take it: `f() if ...; fi`, `f() for ...; done`, a while, an until, a case, and bash's [[ ]]
// and (( )). Only a brace group or a subshell was taken, and the rest stopped the whole script
// before its first line. A redirection after the body is the body's, applied each call.
func TestRuntime_functionBodyIsAnyCompoundCommand(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"f() if true; then echo yes; fi; f", "yes\n"},
		{"f() if true; then\n  echo yes\nfi\nf", "yes\n"},
		{`f() if [ "$1" = a ]; then echo A; else echo B; fi; f a; f b`, "A\nB\n"},
		{"f() for i in 1 2; do echo $i; done; f", "1\n2\n"},
		{"f() while false; do :; done; f; echo $?", "0\n"},
		{"f() until true; do :; done; f; echo done", "done\n"},
		{"f() case $1 in a) echo A;; *) echo other;; esac; f a; f b", "A\nother\n"},
		{"function g() for i in x; do echo $i; done; g", "x\n"},
		{"f() for i in 1; do echo $i; done > /dev/null; f; echo after", "after\n"},
		{"f() [[ -n x ]]; f && echo ok", "ok\n"},
		{"f() (( 1 + 1 )); f && echo ok", "ok\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
