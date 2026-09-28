package runtime_test

import "testing"

// An expansion's parentheses are its own to the scan that finds a line's top-level operators.
// With `$((1+1))` as a case pattern in a function, the pattern's first `(` was taken for a
// pattern's and its second `)` for a subshell's close. The body's depth fell to nothing, and
// the `&&` in the arm was read as the definition's own, so the function was cut there:
// "missing }". Both references print every line, measured.
func TestTopLevelOperators_skipAnExpansionsParentheses(t *testing.T) {
	script := "f ()\n{\n\tcase \"$c,$p\" in\n\t$((1+1)),*|*,-*)\n\t\techo hit && return\n\tesac\n}\nc=2; f\n" +
		"g() { case 3 in $((1+2))) echo three && return;; esac; }; g\n" +
		"h() { x=$(echo a) && echo \"$x\"; }; h\n"
	want := "hit\nthree\na\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
