package main

import (
	"strings"
	"testing"
)

// The bash name is bash mode, and NEMOSH_COMPAT=bash in the environment gives it to the nemosh
// name; sh stays busybox's whatever the variable says. Until something consults the mode, it
// shows only as bash's version, and $0 stays the name the shell was run by.
func TestRun_NEMOSH_COMPATgivesTheNemoshNameBashMode(t *testing.T) {
	report := `echo "$0 ${BASH_VERSION-unset}"`
	for _, test := range []struct {
		name, compat string
		args         []string
		want         string
	}{
		{name: "nemosh with it", compat: "bash", args: []string{"nemosh", "-c", report}, want: "nemosh 5.2.37(1)-release\n"},
		{name: "nemosh without it", args: []string{"nemosh", "-c", report}, want: "nemosh unset\n"},
		{name: "sh with it", compat: "bash", args: []string{"nemosh", "sh", "-c", report}, want: "sh unset\n"},
		{name: "bash without it", args: []string{"nemosh", "bash", "-c", report}, want: "bash 5.2.37(1)-release\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.compat != "" {
				t.Setenv("NEMOSH_COMPAT", test.compat)
			}
			got := runArgs(t, "", test.args...)
			if got.status != 0 || got.stdout != test.want || got.stderr != "" {
				t.Fatalf("status %d, stdout %q, stderr %q; want %q", got.status, got.stdout, got.stderr, test.want)
			}
		})
	}
}

// A value the variable does not know is said, since a misspelt one would otherwise leave the
// shell as it was without a word; the shell goes on as nemosh.
func TestRun_NEMOSH_COMPATsaysAValueItDoesNotKnow(t *testing.T) {
	t.Setenv("NEMOSH_COMPAT", "Bash")
	got := runArgs(t, "", "nemosh", "-c", `echo "${BASH_VERSION-unset}"`)
	if got.status != 0 || got.stdout != "unset\n" || !strings.Contains(got.stderr, "NEMOSH_COMPAT=Bash") {
		t.Fatalf("status %d, stdout %q, stderr %q; want unset, and the value named", got.status, got.stdout, got.stderr)
	}
}
