package runtime_test

import "testing"

// A function's name may hold an `=` that is not an assignment's, one after something that is
// not a name, as busybox-w32 and bash both take it; `a=b() {...}` stays the syntax error it is
// in both. Every `=` was refused, so the definition was `unexpected (`.
func TestRuntime_functionNameWithEquals(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"func-name=ext ( ) { echo func-name=ext; }\nfunc-name=ext\n", "func-name=ext\n", 0},
		{"my-fn=x() { echo g; }; my-fn=x\n", "g\n", 0},
		{"a=b() { echo ab; }; a=b\n", "", 2},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
