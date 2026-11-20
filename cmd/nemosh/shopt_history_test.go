package main

import (
	"strings"
	"testing"
)

// bash's five history shopts are recorded and change nothing here, and a session says why. The
// reason was "there is no history here to change", which was wrong for all five: history is
// kept, in a file a command at a time (history_file.go), with bash's expansion. Ubuntu's
// skel.bashrc sets histappend, so it is the one an administrator meets first.
func TestSession_saysWhyTheHistoryShoptsChangeNothing(t *testing.T) {
	result := runInvocation(t, "", "--norc", "-i", "-c",
		"shopt -s histappend lithist histreedit histverify; shopt -u cmdhist")

	for _, want := range []string{
		"shopt: histappend: recorded, and changes nothing here: the history file is appended to a command at a time either way, so no session overwrites another's\n",
		"shopt: lithist: recorded, and changes nothing here: a command of several lines is kept as it was typed, its newlines and all, either way\n",
		"shopt: histreedit: recorded, and changes nothing here: a history expansion that fails is reported and its line dropped, never given back to edit\n",
		"shopt: histverify: recorded, and changes nothing here: an expanded line is shown and run at once, never given back to edit first\n",
		"shopt: cmdhist: recorded, and changes nothing here: a command of several lines is one entry either way\n",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, result.stderr)
		}
	}
	if strings.Contains(result.stderr, "no history here") {
		t.Errorf("stderr still says there is no history:\n%s", result.stderr)
	}
}
