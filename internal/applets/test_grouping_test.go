package applets_test

import "testing"

// test's grammar reads a `(` as a group before it looks for a comparison, and a `!` with
// anything after it as a negation, as busybox-w32's primary and nexpr do and bash does too;
// POSIX's rules by argument count come first, so three arguments with a binary operator in
// the middle are always that comparison. The comparison came first everywhere, so `( = )`
// after `-a` compared the two parentheses and `! = !` in a longer expression was compared too.
func TestTest_groupsBeforeComparingAndNegatesAnyBang(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int
	}{
		{args: []string{"(", "=", ")"}, want: 1},
		{args: []string{"0", "-eq", "0", "-a", "(", "=", ")"}, want: 0},
		{args: []string{"x", "-a", "(", "=", ")"}, want: 0},
		{args: []string{"(", "=", ")", "-a", "x"}, want: 0},
		{args: []string{"(", "(", "=", ")", ")"}, want: 0},
		{args: []string{"!", "(", "=", ")"}, want: 0},
		{args: []string{"(", "!", "=", ")"}, want: 1},
		{args: []string{"x", "=", "x", "-a", "!"}, want: 0},
		{args: []string{"!", "=", "!", "-a", "x"}, want: 2},
		{args: []string{"x", "-a", "!", "=", "!"}, want: 2},
		{args: []string{"x", "-a", "("}, want: 2},
		{args: []string{"(", "=", "(", "-a", "x"}, want: 2},
		// POSIX's four-argument `( $2 $3 )` is the two-argument test, as bash answers.
		// busybox-w32 says "closing paren expected": its test_main strips the parentheses and
		// then parses from where it began.
		{args: []string{"(", "-n", "x", ")"}, want: 0},
	} {
		if status, stderr := runTestApplet(t, test.args...); status != test.want {
			t.Errorf("test %q = %d, want %d (stderr %q)", test.args, status, test.want, stderr)
		}
	}
}
