package applets_test

import "testing"

// A floating conversion reads its operand as C's strtod reads it, and the whole operand has to
// be read, as in both references: a blank operand, a blank after the number and Go's digit
// separator are no number, said and written as zero, status 1, where they were 0, 3.5 and 10
// with status 0; a hex number may leave out its exponent, `0x1.8` and `0x10`, where it was no
// number. A format that ends inside a conversion, `a%` or `%5`, writes what came before it and
// fails there, status 1, where it was written as it stood. Each answer was measured against
// busybox-w32 and bash 5.3, which agree on all of them but what an unreadable number is written
// as -- zero in busybox -- and the words.
func TestPrintf_readsAFloatAsStrtodAndRefusesAnUnfinishedConversion(t *testing.T) {
	for _, test := range []struct {
		args               []string
		out, said, failure string
	}{
		{args: []string{"[%f]", " "}, out: "[0.000000]", said: "printf: invalid number ' '\n", failure: "exit status 1"},
		{args: []string{"[%f]", "3.5 "}, out: "[0.000000]", said: "printf: invalid number '3.5 '\n", failure: "exit status 1"},
		{args: []string{"[%f]", "1_0"}, out: "[0.000000]", said: "printf: invalid number '1_0'\n", failure: "exit status 1"},
		{args: []string{"[%f]", "0x"}, out: "[0.000000]", said: "printf: invalid number '0x'\n", failure: "exit status 1"},
		{args: []string{"[%f|%f|%f]", "0x1.8", "0x10", "0x1p1"}, out: "[1.500000|16.000000|2.000000]"},
		{args: []string{"[%f|%f|%g]", " 3.5", "+.5e1", "infinity"}, out: "[3.500000|5.000000|inf]"},
		{args: []string{"[%f]", ""}, out: "[0.000000]"},
		{args: []string{"a%"}, out: "a", failure: "%: invalid format"},
		{args: []string{"%5"}, out: "", failure: "%5: invalid format"},
		{args: []string{"b%5", "1", "2"}, out: "b", failure: "%5: invalid format"},
	} {
		out, stderr, err := runSmall(t, t.TempDir(), "", "printf", test.args...)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if out != test.out || stderr != test.said || failure != test.failure {
			t.Errorf("printf %q = %q, %q, %q; want %q, %q, %q", test.args, out, stderr, failure, test.out, test.said, test.failure)
		}
	}
}
