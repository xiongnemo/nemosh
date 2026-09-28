package runtime_test

import "testing"

// `function name { ... }` may hold parentheses in its body: a command substitution, backquoted
// or not, and a subshell. The first `(` on the line was taken for the name's, so the definition
// was read as a command and refused, "unexpected {". Both references print every line,
// measured.
func TestFunctionKeyword_aBodyMayHoldParentheses(t *testing.T) {
	script := "function f { echo $(echo hi); }; f\nfunction g {\n x=`echo D`\n echo \"$x\"\n}\ng\n" +
		"function h { if [ -z \"$(echo)\" ]; then echo empty; fi; }; h\nfunction k { (echo sub); }; k\n" +
		"function m () { echo $(echo paren); }; m\n"
	want := "hi\nD\nempty\nsub\nparen\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
