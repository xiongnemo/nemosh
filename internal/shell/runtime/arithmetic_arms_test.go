package runtime_test

import "testing"

// A parenthesised expression and the middle arm of `?:` are whole expressions, assignments
// and commas included, and only the arm of `?:` the condition picks is evaluated. `(x = 22)`
// and `1 ? a=1 : 42` were syntax errors, and `1 ? 2 : 1/0` refused the division it was never
// going to do. busybox-w32 and bash 5.3 agree on every answer here.
func TestRuntime_arithmeticArmsAreWholeExpressions(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x=11; echo $(( 0 || (x=33) )) $x`, "1 33\n"},
		{`echo $(( (1, 2) )) $(( (x=4) + 1 )) $x`, "2 5 4\n"},
		{`echo $((1 ? a=1 : 42 )); echo a=$a`, "1\na=1\n"},
		{`y=5; echo $(( 0 ? (y=1) : 2 )) $y; echo $(( 1 ? 3 : (y=9) )) $y`, "2 5\n3 5\n"},
		{`echo $(( 1 ? 2 : 1/0 )) $(( 0 ? 1/0 : 3 )) $(( 0 ? 2 ** -1 : 3 ))`, "2 3 3\n"},
		{`x=3; echo $(( 0 ? x += 5 : x )) $x`, "3 3\n"},
		{`echo $(( 0 ? 1 : 2 ? 3 : 4 )) $(( 1 ? 0 ? 5 : 6 : 7 ))`, "3 6\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
	// A syntax error in the arm not taken is still one.
	if stdout, status := runScriptCapturing(`echo $(( 1 ? 2 : )); echo after`); stdout != "" || status == 0 {
		t.Errorf("got %q/%d, want the syntax error to end the script", stdout, status)
	}
}
