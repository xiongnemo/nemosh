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

// A leading quote makes a numeric operand the code of the character after it, POSIX's rule:
// `printf '%d' "'A"` is 65 in both references. It was a non-number, 0 with a diagnostic.
func TestPrintf_readsACharacterCode(t *testing.T) {
	stdout, _, err := runAppletWithInput(t, "", "printf", `%d %d %x %o %.1f|%d`, "'A", `"a`, "'z", "'0", "'A", "'")
	if err != nil || stdout != "65 97 7a 60 65.0|0" {
		t.Fatalf("printf = %q (err %v)", stdout, err)
	}
}
