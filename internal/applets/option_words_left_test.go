package applets_test

import (
	"strings"
	"testing"
)

// bc, dc, getopt, head, tail and nproc read their options a word at a time, and said
// `unsupported option: -Y`, head and tail with a list of what they do take. They say getopt's
// words now, as both references say them on Windows and the other applets already did: a long
// option named whole, a short one by its letter. Measured against busybox-w32.
func TestApplets_wordAtATimeSayGetoptsWords(t *testing.T) {
	for _, test := range []struct {
		applet string
		arg    string
		want   string
	}{
		{"bc", "--bogus", "unknown option -- bogus"},
		{"bc", "-Y", "unknown option -- Y"},
		{"dc", "-Y", "unknown option -- Y"},
		{"getopt", "--bogus", "unknown option -- bogus"},
		{"head", "-Y", "unknown option -- Y"},
		{"tail", "--bogus", "unknown option -- bogus"},
		{"nproc", "-Y", "unknown option -- Y"},
	} {
		_, stderr, err := runSmall(t, t.TempDir(), "", test.applet, test.arg)
		said := stderr
		if err != nil {
			said += err.Error()
		}
		if err == nil || !strings.Contains(said, test.want) || strings.Contains(said, "unsupported") {
			t.Errorf("%s %s said %q, %v; want %q", test.applet, test.arg, said, err, test.want)
		}
	}
}
