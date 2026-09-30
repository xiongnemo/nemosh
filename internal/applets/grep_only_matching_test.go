package applets_test

import "testing"

// grep -o prints no empty match, as busybox's does not: `grep -o '[0-9]*'` is the numbers. It
// printed an empty line for each place the pattern matched nothing. A line that matched only so
// still counts, so -c and the status are as they were. Each answer is busybox-w32's, measured.
func TestGrep_printsNoEmptyMatchUnderO(t *testing.T) {
	input := "b 2\na 10\nc 1\na 10\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-o", "[0-9]*"}, "2\n10\n1\n10\n"},
		{[]string{"-o", "a*"}, "a\na\n"},
		{[]string{"-o", "1*0*"}, "10\n1\n10\n"},
		{[]string{"-on", "[0-9]*"}, "1:2\n2:10\n3:1\n4:10\n"},
		{[]string{"-o", "x*"}, ""},
		{[]string{"-oc", "x*"}, "4\n"},
	} {
		stdout, _, err := runAppletWithInput(t, input, "grep", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("grep %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}
