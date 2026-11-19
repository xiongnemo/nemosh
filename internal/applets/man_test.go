package applets_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
)

// man is this shell's own help, as man-db would put it: `man ls` is what `help ls` says, man -k
// and apropos search the names and summaries in man-db's `NAME (1)  - description` layout, man -f
// and whatis the name alone, and a page there is not is man-db's words and status 16. info, which
// falls back to a man page where there is no Info manual, is the same. None was here.
func TestMan_isThisShellsHelp(t *testing.T) {
	want, _ := capability.UsageFor("ls")
	dir := t.TempDir()
	for _, applet := range []string{"man", "info"} {
		if stdout, stderr, err := runSmall(t, dir, "", applet, "ls"); err != nil || stdout != want {
			t.Errorf("%s ls = %q, %v (%s); want ls's help", applet, stdout, err, stderr)
		}
	}
	if stdout, _, err := runSmall(t, dir, "", "man", "1", "ls"); err != nil || stdout != want {
		t.Errorf("man 1 ls = %q, %v", stdout, err)
	}
	line := "ls (1)               - list directory contents"
	if !strings.Contains(want, "\nList directory contents.\n") {
		t.Fatalf("ls's summary moved; this test reads it: %q", want)
	}
	for _, test := range []struct {
		applet string
		args   []string
	}{
		{"whatis", []string{"ls"}}, {"man", []string{"-f", "ls"}},
	} {
		if stdout, _, err := runSmall(t, dir, "", test.applet, test.args...); err != nil || stdout != line+"\n" {
			t.Errorf("%s %q = %q, %v; want %q", test.applet, test.args, stdout, err, line)
		}
	}
	for _, args := range [][]string{{"-k", "^ls$"}, {"-k", "LIST DIRECTORY"}} {
		if stdout, _, err := runSmall(t, dir, "", "man", args...); err != nil || !strings.Contains(stdout, line+"\n") {
			t.Errorf("man %q = %q, %v; want it to list ls", args, stdout, err)
		}
	}
	if stdout, _, err := runSmall(t, dir, "", "apropos", "director"); err != nil || !strings.Contains(stdout, line+"\n") || strings.Count(stdout, "\n") < 2 {
		t.Errorf("apropos director = %q, %v", stdout, err)
	}
	for _, test := range []struct {
		applet string
		args   []string
		says   string
	}{
		{"man", []string{"frobnicate"}, "No manual entry for frobnicate"},
		{"man", []string{"5", "passwd"}, "No manual entry for passwd in section 5"},
		{"apropos", []string{"zzzqqq"}, "zzzqqq: nothing appropriate."},
		{"whatis", []string{"frobnicate"}, "frobnicate: nothing appropriate."},
	} {
		_, _, err := runSmall(t, dir, "", test.applet, test.args...)
		if status, _ := applets.StatusCode(err); err == nil || !strings.Contains(err.Error(), test.says) || status != 16 {
			t.Errorf("%s %q = %v; want %q and status 16", test.applet, test.args, err, test.says)
		}
	}
	if _, _, err := runSmall(t, dir, "", "man"); err == nil || !strings.Contains(err.Error(), "What manual page do you want?") {
		t.Errorf("man alone = %v", err)
	}
}
