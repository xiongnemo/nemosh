package runtime_test

import "testing"

// unset with a name no variable can have is busybox's error, in its words, and ends the
// script as a special builtin's error does there; a subshell ends only itself. It returned
// 0 having done nothing, where bash says so too, with 1. The names before it are unset.
func TestUnset_refusesANameNoVariableCanHave(t *testing.T) {
	for _, test := range []struct{ name, script, stdout, stderr string }{
		{name: "an operator in it", script: "unset a-b\necho after\n", stderr: "nemosh: line 1: unset: a-b: bad variable name\n"},
		{name: "the empty name", script: "unset ''\necho after\n", stderr: "nemosh: line 1: unset: : bad variable name\n"},
		{name: "the names before it go", script: "x=1\nunset x 1y\necho after\n", stderr: "nemosh: line 2: unset: 1y: bad variable name\n"},
		{name: "a subshell ends only itself", script: "(unset %)\necho \"after $?\"\n", stdout: "after 2\n", stderr: "nemosh: line 1: unset: %: bad variable name\n"},
		{name: "an element and a function are names", script: "a=(1 2)\nunset 'a[0]' a[1]\nf() { :; }\nunset -f f a-b\necho \"after $? ${#a[@]}\"\n", stdout: "after 0 0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			wantStatus := 2
			if test.stdout != "" {
				wantStatus = 0
			}
			if status != wantStatus || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("got %d/%q/%q, want %d/%q/%q", status, stdout, stderr, wantStatus, test.stdout, test.stderr)
			}
		})
	}
}
