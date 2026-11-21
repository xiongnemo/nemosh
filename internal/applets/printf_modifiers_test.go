package applets_test

import "testing"

// C's length modifiers are read and passed over, as both references pass over the ones they
// know: h, l, L and z, doubled or not, before a number, %c or %s in busybox-w32's and bash 5.3's
// printf, and j and t, and any of them before %b and %q, in bash's, which only bash takes. Each
// was an invalid conversion specification, status 1, so a format written for C, `%ld`, printed
// nothing from there on.
func TestPrintf_passesOverLengthModifiers(t *testing.T) {
	for _, test := range []struct {
		args    []string
		out     string
		failure string
	}{
		{args: []string{"%ld|%hhd|%zu|%jd|%td|%Lf|%llx|%lu\n", "5", "6", "7", "8", "9", "1.5", "255", "3"}, out: "5|6|7|8|9|1.500000|ff|3\n"},
		{args: []string{"[%5ld|%-4hd]", "1", "2"}, out: "[    1|2   ]"},
		{args: []string{"[%lc|%zs|%lb|%jq]", "xy", "s", `a\tb`, "q r"}, out: "[x|s|a\tb|q\\ r]"},
		{args: []string{"[%Zd]", "1"}, out: "[", failure: "invalid conversion specification %Z"},
	} {
		out, _, err := runSmall(t, t.TempDir(), "", "printf", test.args...)
		failure := ""
		if err != nil {
			failure = err.Error()
		}
		if out != test.out || failure != test.failure {
			t.Errorf("printf %q = %q, %q; want %q, %q", test.args, out, failure, test.out, test.failure)
		}
	}
}
