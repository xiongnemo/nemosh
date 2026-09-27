package runtime_test

import (
	"strings"
	"testing"
)

// An arithmetic expression is expanded once. A command substitution that arrives in a
// subscript from a variable's value is not run a second time: `x='$(cmd)'; $(( a[$x] ))`
// ran cmd, which let data a script indexed by become code. bash 5.3 answers with a syntax
// error and runs nothing; busybox has no arrays.
func TestArithmetic_subscriptIsNotExpandedTwice(t *testing.T) {
	for _, script := range []string{
		"x='$(echo PWNED >&2; echo 2)'\na=(10 20 30)\necho $(( a[$x] ))\n",
		"x='$(echo PWNED >&2; echo 2)'\na=(10 20 30)\n(( a[$x] = 5 ))\necho \"${a[@]}\"\n",
		"x='`echo PWNED >&2; echo 2`'\na=(10 20 30)\n(( b = a[$x] ))\n",
	} {
		// When
		_, _, stderr := runSetScript(t, script)

		// Then the command's own line is not there; the diagnostic quotes its text.
		for _, line := range strings.Split(stderr, "\n") {
			if line == "PWNED" {
				t.Fatalf("%q ran the command held in x; stderr = %q", script, stderr)
			}
		}
		if !strings.Contains(stderr, "arithmetic syntax error") {
			t.Fatalf("%q: stderr = %q, want a syntax error", script, stderr)
		}
	}
}

// What is expanded once still works: a plain `$name` in a subscript that has not been
// expanded yet, as in `let`'s quoted operand, and a substitution written in the expression.
func TestArithmetic_subscriptStillExpandsOnce(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"i=1\na=(10 20 30)\nlet 'b=a[$i]'\necho \"$b $(( a[$(echo 2)] )) $(( a[$i] ))\"\n")

	// Then
	if stdout != "20 30 20\n" || status != 0 {
		t.Fatalf("got %q/%d; stderr = %q", stdout, status, stderr)
	}
}
