package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A session has the aliases Ubuntu's ~/.bashrc gives one -- ll, la and l, and ls and the greps in
// colour on a terminal -- before its rc file, which can take them away again. A script has none,
// and NEMOSH_DEFAULTS=off in the environment is a session without them, as a nemosh session was
// until 2026-11-20.
func TestSession_hasUbuntusAliases(t *testing.T) {
	const ubuntu = "egrep='egrep --color=auto'\nfgrep='fgrep --color=auto'\ngrep='grep --color=auto'\n" +
		"l='ls -CF'\nla='ls -A'\nll='ls -alF'\nls='ls --color=auto'\n"
	rc := filepath.Join(t.TempDir(), "rc")
	if err := os.WriteFile(rc, []byte("unalias ll la l\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		defaults string
		args     []string
		stdin    string
		stdout   string
	}{
		{name: "a prompt", args: []string{"--norc", "-i"}, stdin: "alias\n", stdout: ubuntu},
		{name: "-i -c", args: []string{"--norc", "-i", "-c", "alias"}, stdout: ubuntu},
		{name: "ll is ls in colour", args: []string{"--norc", "-i", "-c", "type ll"}, stdout: "ll is an alias for ls -alF\n"},
		{name: "the rc file comes after", args: []string{"--rcfile", rc, "-i", "-c", "alias"},
			stdout: "egrep='egrep --color=auto'\nfgrep='fgrep --color=auto'\ngrep='grep --color=auto'\nls='ls --color=auto'\n"},
		{name: "a script has none", args: []string{"-c", "alias"}, stdout: ""},
		{name: "NEMOSH_DEFAULTS=off", defaults: "off", args: []string{"--norc", "-i"}, stdin: "alias\n", stdout: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NEMOSH_DEFAULTS", test.defaults)
			result := runInvocation(t, test.stdin, test.args...)
			if result.status != 0 || result.stdout != test.stdout {
				t.Fatalf("nemosh %q = %q/%d, want %q; stderr %q", test.args, result.stdout, result.status, test.stdout, result.stderr)
			}
		})
	}
}
