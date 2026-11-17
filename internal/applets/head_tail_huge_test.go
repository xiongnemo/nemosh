package applets_test

import "testing"

// A count is how many lines to keep, not how many to make room for first. `tail -n
// 1000000000000` of a pipe made room for 10^12 lines before reading one, 16 TB, and the whole
// shell died of it -- a Go runtime's out of memory is no panic a recover sees -- and so did
// `head -n -1000000000000`. Each holds what has arrived. The answers are GNU's: busybox-w32 says
// its count is too big for the second, and for a count near 2^32 crashes.
func TestHeadTail_aHugeCountHoldsOnlyWhatArrived(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "tail", args: []string{"-n", "1000000000000"}, want: "a\nb\n"},
		{applet: "tail", args: []string{"-c", "1000000000000"}, want: "a\nb\n"},
		{applet: "head", args: []string{"-n", "-1000000000000"}},
		{applet: "head", args: []string{"-c", "-1000000000000"}},
	} {
		stdout, stderr, err := runSmall(t, t.TempDir(), "a\nb\n", test.applet, test.args...)
		if err != nil || stdout != test.want || stderr != "" {
			t.Errorf("%s %q of a pipe = %q, %q, %v; want %q", test.applet, test.args, stdout, stderr, err, test.want)
		}
	}
}
