package runtime_test

import "testing"

// A binary operator after a term makes it a comparison, whatever the term looks like. With
// x=-f, `[[ $x == $x ]]` was taken as a test of a file named `==` followed by a stray word,
// and `[[ $p == "(" ]]` with p='(' as a group with no end: both were syntax errors, status 2,
// where busybox-w32 and bash compare two strings. A literal `-f == -f` compares too, as
// busybox has it; bash calls that one a syntax error.
func TestRuntime_doubleBracketComparisonComesFirst(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x=-f; [[ $x == $x ]] && echo y`, "y\n"},
		{`p="("; [[ $p == "(" ]] && echo y`, "y\n"},
		{`x=-z; [[ $x != -n ]] && echo y`, "y\n"},
		{`[[ -f == -f ]] && echo y`, "y\n"},
		{`[[ ! == x ]] || echo n`, "n\n"},
		// Unchanged: a unary test, a negation and a group are as they were.
		{`[[ -f /nonexistent/x ]] || echo n`, "n\n"},
		{`[[ ! a == b ]] && echo y`, "y\n"},
		{`[[ ( a == a ) && ! -z x ]] && echo y`, "y\n"},
		{`[[ -n == ]] && echo y`, "y\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
}
