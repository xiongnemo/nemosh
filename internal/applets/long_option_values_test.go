package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Long options are read with the letters, as getopt_long reads them, where each was turned into
// its letter before any was read. So what follows env's PROG is PROG's own: `env echo --null x`
// echoed -0 x. A value given with `=` to one that takes none is refused, where it became an
// operand and `mkdir --parents=yes d` made yes; a missing value is named as it was typed; and a
// prefix two letters share is ambiguous. Each answer was measured against busybox-w32 and
// MSYS's tools, which say "doesn't" where nemosh says "does not".
func TestLongOptions_areReadWithTheLetters(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"g": "x\n"})
	view := permuteTestView{cwd: dir}
	if stdout, stderr, err := runPermuted(t, view, "", "env", "echo", "--null", "x"); stdout != "--null x\n" || err != nil {
		t.Errorf("env echo --null x = %q, %q, %v; want --null x", stdout, stderr, err)
	}
	for _, test := range []struct {
		applet  string
		args    []string
		failure string
	}{
		{applet: "mkdir", args: []string{"--parents=yes", "d"}, failure: "option does not take an argument -- parents"},
		{applet: "mkdir", args: []string{"--par=1", "d"}, failure: "option does not take an argument -- par"},
		{applet: "mkdir", args: []string{"d", "--mode"}, failure: "option requires an argument -- mode"},
		{applet: "cmp", args: []string{"--verbose=1", "g", "g"}, failure: "option does not take an argument -- verbose"},
		{applet: "od", args: []string{"--s", "2", "g"}, failure: "ambiguous option -- s"},
		{applet: "ipcalc", args: []string{"--net", "10.1.1.1/8"}, failure: "ambiguous option -- net"},
		{applet: "ipcalc", args: []string{"--net=1", "10.1.1.1/8"}, failure: "ambiguous option -- net"},
	} {
		_, _, err := runSmall(t, dir, "", test.applet, test.args...)
		if err == nil || err.Error() != test.failure {
			t.Errorf("%s %q = %v; want %q", test.applet, test.args, err, test.failure)
		}
	}
	for _, name := range []string{"yes", "d", "1"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("mkdir made %s, which was no operand", name)
		}
	}
}
