package runtime_test

import "testing"

// A simple command is a function body too, as busybox and dash take it: `f() echo hi` defines
// f, and each call says hi. bash refuses it, and so did this, "function body must be a compound
// command". A brace group and a subshell are read as before, and a word after one is refused.
func TestRuntime_aSimpleCommandMayBeAFunctionBody(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"f() echo hi; f; f\n", "hi\nhi\n"},
		{"f() echo hi >&2; f 2>/dev/null; echo after\n", "after\n"},
		{"f() a=1; f; echo \"a=$a\"\n", "a=1\n"},
		{"f() echo in && echo and; f\n", "and\nin\n"},
		{"f()\necho nl; f\n", "nl\n"},
		{"f() { echo braced; }; f\n", "braced\n"},
		{"f() (echo sub); f\n", "sub\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
	// A word after a braced body is refused, and so is a simple body after `function name`
	// with no parentheses, as busybox refuses both; `function name()` takes one.
	for _, script := range []string{"f() { echo x; } extra\n", "function f echo hi\nf\n"} {
		if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
			t.Errorf("%q: got %q/%d, want it refused, as busybox refuses it", script, stdout, status)
		}
	}
	if stdout, status := runScriptCapturing("function f() echo hi\nf\n"); stdout != "hi\n" || status != 0 {
		t.Errorf("function f() echo hi: got %q/%d, want hi, as busybox answers", stdout, status)
	}
}
