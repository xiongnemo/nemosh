package runtime_test

import "testing"

// A reserved word may be followed by `(` with no blank between: `if(true)`, `do(echo x)`, and
// in bash `while((i < 2))`. Each was one word that was no reserved word, so the `then` or `do`
// after it stood alone and the script was refused. bash prints every line; busybox-w32 agrees
// on each that holds no arithmetic command, which it does not have.
func TestReservedWord_aParenthesisMayFollowIt(t *testing.T) {
	script := "if(true); then echo s; fi\nif((1)); then echo y; fi\nif((1))\nthen\n echo y2\nfi\n" +
		"i=0; while((i<2)); do echo w$i; ((i++)); done\nuntil(false); do echo u; break; done\n" +
		"if false; then :; elif(true); then echo e; fi\nfor d in 1; do(echo d$d); done\n" +
		"if true; then(echo t); fi\nif false; then :; else(echo l); fi\n"
	want := "s\ny\ny2\nw0\nw1\nu\ne\nd1\nt\nl\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, want)
	}
}
