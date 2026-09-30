package applets_test

import "testing"

// A 0 and octal digits in a program's text is octal, as busybox's awk and gawk read it: `010` is
// 8. It was 10. A digit or a point after the run makes it decimal, as busybox's falls back to
// strtod, and a string or field that holds 010 is still 10. Each answer is busybox-w32's,
// measured.
func TestAwk_readsAZeroLedConstantAsOctal(t *testing.T) {
	for _, test := range []struct{ program, input, want string }{
		{`BEGIN { print 010, 08, 010.5, 0x10, 00, 0777, -010, 010e1 }`, "", "8 8 10.5 16 0 511 -8 8\n"},
		{`BEGIN { x = "010"; print x + 0, 1+010 }`, "", "10 9\n"},
		{`{ print $1 + 0 }`, "010\n", "10\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "awk", test.program)
		if err != nil || stdout != test.want {
			t.Errorf("awk %q = %q (err %v), want %q", test.program, stdout, err, test.want)
		}
	}
}
