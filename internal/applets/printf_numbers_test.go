package applets

import (
	"strings"
	"testing"
)

// printf's numbers and its \c, as busybox's printf has them -- C's, in every case measured.
// A number is what strtoll reads, with nothing after its digits; an unsigned conversion
// takes the whole unsigned range and a negative number as its bits; # puts no 0x on a zero;
// and \c ends all output, the rest of the format and every operand still to come.
func TestPrintf_numbersAndStopAsBusyboxHasThem(t *testing.T) {
	for _, test := range []struct {
		args   []string
		want   string
		failed bool
	}{
		{args: []string{"%d|", " -123", " +077", " +0xff"}, want: "-123|63|255|"},
		{args: []string{"%d|", " -123 "}, want: "0|", failed: true},
		{args: []string{"%d|", "0b11"}, want: "0|", failed: true},
		{args: []string{"%d|", "1_000"}, want: "0|", failed: true},
		{args: []string{"%d|", "0x"}, want: "0|", failed: true},
		{args: []string{"%d|", "9223372036854775807", "-9223372036854775808"}, want: "9223372036854775807|-9223372036854775808|"},
		{args: []string{"%d|", "9223372036854775808"}, want: "0|", failed: true},
		{args: []string{"[%u][%o][%x][%X]", "-42", "-42", "-42", "-42"}, want: "[18446744073709551574][1777777777777777777726][ffffffffffffffd6][FFFFFFFFFFFFFFD6]"},
		{args: []string{"%u|", "18446744073709551615"}, want: "18446744073709551615|"},
		{args: []string{"%u|", "18446744073709551616"}, want: "0|", failed: true},
		{args: []string{"%u|", "-18446744073709551615"}, want: "0|", failed: true},
		{args: []string{"[%#o][%#x][%#X][%#x]", "0", "0", "0", "42"}, want: "[0][0][0][0x2a]"},
		{args: []string{"%o|", "8"}, want: "10|"},
		{args: []string{"[%b]\n", `ab\ncd\cef`}, want: "[ab\ncd"},
		{args: []string{`%s\c`, "x", "y"}, want: "x"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			var stdout, stderr strings.Builder
			applet, _ := DefaultRegistry.Lookup("printf")
			err := applet.Run(t.Context(), test.args, strings.NewReader(""), &stdout, &stderr)
			if stdout.String() != test.want || (err != nil) != test.failed {
				t.Fatalf("printf %q = %q (%v, %s), want %q, failed %v", test.args, stdout.String(), err, stderr.String(), test.want, test.failed)
			}
		})
	}
}

// printf with nothing at all is a usage error, 2, as in busybox.
func TestPrintf_withNoArgumentsIsStatusTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	applet, _ := DefaultRegistry.Lookup("printf")
	err := applet.Run(t.Context(), nil, strings.NewReader(""), &stdout, &stderr)
	if code, ok := StatusCode(err); !ok || code != 2 {
		t.Fatalf("printf with no arguments: %v, want status 2", err)
	}
}
