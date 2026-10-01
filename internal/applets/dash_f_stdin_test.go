package applets_test

import (
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `-f -` reads the patterns, the script or the program from standard input, as busybox's grep,
// sed and awk read it: `... | grep -f - file`. It was a file named -, not there. With no FILE
// to read, standard input has gone to the -f and there is nothing left. Each answer is
// busybox-w32's, measured; GNU grep 3.0 agrees with its grep.
func TestDashF_readsStandardInput(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "banana 3\napple 10\ncherry 2\napple 10\nDate 7\n\nelder 1\n"})
	for _, test := range []struct {
		applet    string
		stdin     string
		args      []string
		want      string
		noneFound bool
	}{
		{applet: "grep", stdin: "apple\n", args: []string{"-f", "-", "a.txt"}, want: "apple 10\napple 10\n"},
		{applet: "grep", stdin: "x\n", args: []string{"-e", "banana", "-f", "-", "a.txt"}, want: "banana 3\n"},
		{applet: "grep", stdin: "ban\napp\n", args: []string{"-c", "-f", "-", "a.txt"}, want: "3\n"},
		{applet: "grep", stdin: "", args: []string{"-f", "-", "a.txt"}, noneFound: true},
		{applet: "grep", stdin: "apple\n", args: []string{"-f", "-"}, noneFound: true},
		{applet: "sed", stdin: "2p\n", args: []string{"-n", "-f", "-", "a.txt"}, want: "apple 10\n"},
		{applet: "awk", stdin: "NR < 3 { print $1 }\n", args: []string{"-f", "-", "a.txt"}, want: "banana\napple\n"},
	} {
		got, _, err := runSmall(t, dir, test.stdin, test.applet, test.args...)
		if got != test.want || test.noneFound != errors.Is(err, applets.ErrExitFalse) || !test.noneFound && err != nil {
			t.Errorf("%s %q of %q = %q, %v; want %q", test.applet, test.args, test.stdin, got, err, test.want)
		}
	}
}
