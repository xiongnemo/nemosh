package runtime

import "testing"

// [[ reads a number as [ does, and names the operand that is none, as busybox's [[ does: `[[ x
// -lt 1 ]]` is `x: bad number`, status 2. It named the operator, `-lt: integer expression
// expected`, and refused the blanks around a number that [ and both references take.
func TestDoubleBracket_readsANumberAsBracketDoes(t *testing.T) {
	for _, test := range []struct {
		script, stdout, stderr string
	}{
		{script: "[[ x -lt 1 ]]; echo \"st=$?\"\n", stdout: "st=2\n", stderr: "nemosh: line 1: [[: x: bad number\n"},
		{script: "[[ 3 -eq e ]]; echo \"st=$?\"\n", stdout: "st=2\n", stderr: "nemosh: line 1: [[: e: bad number\n"},
		{script: "[[ '' -eq 0 ]]; echo \"st=$?\"\n", stdout: "st=2\n", stderr: "nemosh: line 1: [[: bad number\n"},
		{script: "[[ ' 3' -eq 3 ]]; echo \"st=$?\"\n", stdout: "st=0\n"},
		{script: "[ '' -eq 0 ]; echo \"st=$?\"\n", stdout: "st=2\n", stderr: "[: bad number\n"},
	} {
		if stdout, stderr, _ := runKill(t, test.script); stdout != test.stdout || stderr != test.stderr {
			t.Errorf("%q: stdout %q, stderr %q; want %q and %q", test.script, stdout, stderr, test.stdout, test.stderr)
		}
	}
}
