package applets_test

import "testing"

// An operand given empty or blank to an integer conversion is no number, as both references
// read it: "invalid number" with the operand quoted, zero written in its place, and status 1.
// It was a silent zero.
// One that is missing is still zero and no error, as POSIX and bash have it. %c of an empty
// operand, given or missing, is the string's end, a NUL, padded to the width, as both write
// it; it wrote nothing. Each answer was measured against busybox-w32 and bash 5.3.
func TestPrintf_emptyOperands(t *testing.T) {
	for _, test := range []struct {
		args       []string
		out, error string
	}{
		{args: []string{"%d\n", ""}, out: "0\n", error: "printf: invalid number ''\n"},
		{args: []string{"%x|%u\n", "", ""}, out: "0|0\n", error: "printf: invalid number ''\nprintf: invalid number ''\n"},
		{args: []string{"%d|%d\n", "1"}, out: "1|0\n"},
		{args: []string{"%f\n", ""}, out: "0.000000\n"},
		{args: []string{"%c|", "abc", "", "x"}, out: "a|\x00|x|"},
		{args: []string{"%c|"}, out: "\x00|"},
		{args: []string{"%5c|", ""}, out: "    \x00|"},
	} {
		out, stderr, err := runSmall(t, t.TempDir(), "", "printf", test.args...)
		if out != test.out || stderr != test.error || (err != nil) != (test.error != "") {
			t.Errorf("printf %q = %q, %q, %v; want %q, %q", test.args, out, stderr, err, test.out, test.error)
		}
	}
}
