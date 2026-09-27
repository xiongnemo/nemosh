package applets_test

import "testing"

// egrep is grep -E and fgrep is grep -F, as busybox-w32 has them (grep_main reads the -E or
// -F off the applet's name). Neither was a command here, so a script that used one -- Oils
// does, a dozen times, to check what a variable holds -- ran nothing. Each answer is
// busybox-w32's, measured.
func TestGrep_egrepAndFgrepAreItsEAndF(t *testing.T) {
	for _, test := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{name: "egrep", input: "a+b\naab\n", args: []string{"a+b"}, want: "aab\n"},
		{name: "egrep", input: "x1\nx\ny2\n", args: []string{"-o", "[0-9]+"}, want: "1\n2\n"},
		{name: "egrep", input: "ab\ncd\n", args: []string{"ab|cd"}, want: "ab\ncd\n"},
		{name: "fgrep", input: "a+b\naab\n", args: []string{"a+b"}, want: "a+b\n"},
		{name: "fgrep", input: "a.c\nabc\n", args: []string{"-c", "a.c"}, want: "1\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, test.name, test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("%s %q = %q (err %v), want %q", test.name, test.args, stdout, err, test.want)
		}
	}
}
