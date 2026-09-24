package applets_test

import "testing"

// A `*` takes the width or the precision from the next operand. Each answer is
// busybox-w32's, measured; it was an invalid conversion specification.
func TestPrintf_takesWidthAndPrecisionFromOperands(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{`%*d|`, "5", "42"}, want: "   42|"},
		{args: []string{`%-*s|%.*f`, "6", "ab", "2", "3.14159"}, want: "ab    |3.14"},
		{args: []string{`%*s|`, "-4", "x"}, want: "x   |"},
		{args: []string{`%.*f|`, "-1", "2.5"}, want: "2.500000|"},
		{args: []string{`%*d|`, "3", "1", "4", "2"}, want: "  1|   2|"},
		{args: []string{`%0*d`, "5", "7"}, want: "00007"},
	} {
		stdout, _, err := runAppletWithInput(t, "", "printf", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("printf %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}
