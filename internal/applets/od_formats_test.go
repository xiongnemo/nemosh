package applets_test

import "testing"

// Several -t formats are each a line of their own over the same bytes, every field as wide
// as the widest format's, and the lines after the first indented as far as the address. Only
// the last -t was used, so `od -A n -t c -t x1` -- the way to see a byte both as a character
// and as a number -- printed the numbers alone. The layout is GNU od's, which Oils records:
// busybox-w32's puts some seventy blanks before the second line under -A n, which is no
// layout at all, and with an address leaves each format at its own width.
func TestOd_printsEachFormatOnALineOfItsOwn(t *testing.T) {
	for _, test := range []struct {
		input string
		args  []string
		want  string
	}{
		{input: "\x00-\n", args: []string{"-A", "n", "-t", "c", "-t", "x1"}, want: "  \\0   -  \\n\n  00  2d  0a\n"},
		{input: "ab", args: []string{"-t", "c", "-t", "x1"}, want: "0000000   a   b\n         61  62\n0000002\n"},
		{input: "ab", args: []string{"-A", "x", "-t", "x1", "-t", "c"}, want: "000000  61  62\n         a   b\n000002\n"},
		{input: "ab", args: []string{"-A", "n", "-t", "x1"}, want: " 61 62\n"},
		// -A x is six hex digits and -A d decimal, in busybox and GNU alike. They were seven
		// hex digits both.
		{input: "0123456789abcdef!", args: []string{"-A", "d", "-t", "x1"}, want: "0000000 30 31 32 33 34 35 36 37 38 39 61 62 63 64 65 66\n0000016 21\n0000017\n"},
		{input: "0123456789abcdef!", args: []string{"-A", "x", "-t", "x1"}, want: "000000 30 31 32 33 34 35 36 37 38 39 61 62 63 64 65 66\n000010 21\n000011\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "od", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("od %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}
