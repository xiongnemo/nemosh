package applets_test

import "testing"

// s///p writes the pattern space when a replacement was made, and only then, as busybox's and
// POSIX's do: `sed -n 's/a/X/p'` is the lines that had an a. The p was not taken for a flag, so
// it was the next command, a print of every line. A replacement that leaves the text as it was,
// `s/a/a/`, is still one, to p and to t. Each answer is busybox-w32's, measured.
func TestSed_substituteFlagPPrintsOnlyWhatItChanged(t *testing.T) {
	input := "a\nb\naa\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-n", "s/a/X/p"}, "X\nXa\n"},
		{[]string{"-n", "s/a/X/gp"}, "X\nXX\n"},
		{[]string{"s/a/X/p"}, "X\nX\nb\nXa\nXa\n"},
		{[]string{"-n", "s/a/a/p"}, "a\naa\n"},
		{[]string{"-n", "s/a/X/2p"}, "aX\n"},
		{[]string{"-n", "s/x/y/p"}, ""},
		{[]string{"s/a/a/;t;s/$/-no/"}, "a\nb-no\naa\n"},
	} {
		stdout, _, err := runAppletWithInput(t, input, "sed", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}
