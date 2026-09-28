package runtime_test

import "testing"

// A comment may follow an operator with no blank between: `echo a;# c`, `echo b &# c`, `echo c
// |# c`. The `#` begins a word there, and so a comment, as both references read it; nemosh ran
// a command called `#`. After the `(` of an extended pattern it is still the pattern's, as in
// bash's `[[ "#a" == @(#*) ]]`. Both references print every line, the last bash alone,
// measured.
func TestComment_mayFollowAnOperator(t *testing.T) {
	script := "echo a;# c\necho b &# c\nwait\necho c |# c\ncat\nx=1;#y\necho \"$x\"\n" +
		"[[ \"#a\" == @(#*) ]] && echo pat\n"
	want := "a\nb\nc\n1\npat\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
