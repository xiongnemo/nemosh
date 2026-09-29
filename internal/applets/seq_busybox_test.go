package applets_test

import "testing"

// seq answers as busybox-w32's does, each case measured there: fractional steps with the
// decimals of every operand but LAST, -w padding to the widest integer part, -s between the
// numbers, a negative FIRST read as a number and not an option, and hexadecimal and exponents as
// strtod reads them. It read integers alone and took no options.
func TestSeq_answersAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-s,", "3"}, "1,2,3\n"},
		{[]string{"-w", "8", "10"}, "08\n09\n10\n"},
		{[]string{"1", "0.5", "2"}, "1.0\n1.5\n2.0\n"},
		// Each number is FIRST plus a whole number of INCs, and 0.1 + 2*0.1 is past 0.3.
		{[]string{"0.1", "0.1", "0.3"}, "0.1\n0.2\n"},
		{[]string{"-1", "1"}, "-1\n0\n1\n"},
		{[]string{"-s", ":", "-1", "1"}, "-1:0:1\n"},
		{[]string{"5", "1"}, ""},
		{[]string{"1", "2.0"}, "1\n2\n"},
		{[]string{"1.0", "2"}, "1.0\n2.0\n"},
		{[]string{"-w", "-1", "1"}, "-1\n00\n01\n"},
		{[]string{"-w", "-10", "5", "0"}, "-10\n-05\n000\n"},
		{[]string{"-s", "", "3"}, "123\n"},
		{[]string{"10", "-2", "1"}, "10\n8\n6\n4\n2\n"},
		{[]string{"1.5"}, "1\n"},
		{[]string{"2", "1.5", "5"}, "2.0\n3.5\n5.0\n"},
		{[]string{"-w", "1", "0.25", "1.5"}, "1.00\n1.25\n1.50\n"},
		{[]string{"3", "-1", "1.5"}, "3\n2\n"},
		{[]string{"1", " 2"}, "1\n2\n"},
		{[]string{"-1.5", "1"}, "-1.5\n-0.5\n0.5\n"},
		{[]string{"-w", "0x3"}, "001\n002\n003\n"},
		{[]string{"-w", "1e1", "2", "14"}, "010\n012\n014\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"seq"}, test.args...)...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("seq %q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	for _, args := range [][]string{{"x"}, {"1", "2 "}, {"1", "2", "3", "4"}, {"-Z", "3"}, {}, {"1", "0", "5"}} {
		if _, _, err := runPermuted(t, view, "", append([]string{"seq"}, args...)...); err == nil {
			t.Errorf("seq %q succeeded", args)
		}
	}
}
