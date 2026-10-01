package applets_test

import (
	"testing"
)

// tail -n +N and head -n -N keep each line's ending, as tail -n N and head -n N already did: a
// last line without a newline is written without one, and a CRLF stays a CRLF. Each answer is
// busybox-w32's, measured; a newline was added, and a CRLF made an LF.
func TestHeadTail_countsFromTheOtherEndKeepEndings(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"nonl": "a\nb", "crlf": "one\r\ntwo\r\n"})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "tail", args: []string{"-n", "+1", "nonl"}, want: "a\nb"},
		{applet: "tail", args: []string{"-n", "+2", "nonl"}, want: "b"},
		{applet: "tail", args: []string{"-n", "+2", "crlf"}, want: "two\r\n"},
		{applet: "head", args: []string{"-n", "-1", "crlf"}, want: "one\r\n"},
		{applet: "head", args: []string{"-n", "-0", "nonl"}, want: "a\nb"},
		{applet: "head", args: []string{"-n", "-1", "nonl"}, want: "a\n"},
	} {
		if got, _, err := runSmall(t, dir, "", test.applet, test.args...); got != test.want || err != nil {
			t.Errorf("%s %q = %q, %v; want %q", test.applet, test.args, got, err, test.want)
		}
	}
}
