package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A session exports the LS_COLORS Ubuntu's ~/.bashrc gets from `eval "$(dircolors -b)"`, coreutils
// 9.4's database, so `ls --color=auto` colours an archive red and a picture magenta as there. One
// the environment set is kept; a script has none of its own; NEMOSH_DEFAULTS=off sets none.
func TestSession_exportsDircolorsDatabase(t *testing.T) {
	t.Setenv("LS_COLORS", "")
	os.Unsetenv("LS_COLORS")
	report := `printenv LS_COLORS | cut -c1-44; printenv LS_COLORS | tr : '\n' | grep -c .`
	for _, test := range []struct {
		name, defaults, inherited string
		args                      []string
		stdout                    string
	}{
		{name: "a session", args: []string{"--norc", "-i", "-c", report}, stdout: "rs=0:di=01;34:ln=01;36:mh=00:pi=40;33:so=01;\n151\n"},
		{name: "one inherited is kept", inherited: "di=01;31", args: []string{"--norc", "-i", "-c", report}, stdout: "di=01;31\n1\n"},
		{name: "a script", args: []string{"-c", report}, stdout: "0\n"},
		{name: "NEMOSH_DEFAULTS=off", defaults: "off", args: []string{"--norc", "-i", "-c", report}, stdout: "0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NEMOSH_DEFAULTS", test.defaults)
			if test.inherited != "" {
				t.Setenv("LS_COLORS", test.inherited)
			}
			result := runInvocation(t, "", test.args...)
			if result.stdout != test.stdout {
				t.Fatalf("nemosh %q = %q, want %q; stderr %q", test.args, result.stdout, test.stdout, result.stderr)
			}
		})
	}
}

// Before each prompt a session has COLUMNS and LINES, the terminal's size, as bash's checkwinsize
// has them, so `echo $COLUMNS` answers at a prompt and a script started from one can read it.
// `shopt -u checkwinsize` leaves them as they are, and a script has neither. checkwinsize was
// recorded and did nothing.
func TestSession_hasTheTerminalsSize(t *testing.T) {
	t.Setenv("COLUMNS", "")
	os.Unsetenv("COLUMNS")
	t.Setenv("LINES", "")
	os.Unsetenv("LINES")
	run := func(stdin string, args ...string) string {
		var stdout, stderr bytes.Buffer
		cmd := command{stdin: strings.NewReader(stdin), stdout: &stdout, stderr: &stderr,
			windowSize: func() (int, int, bool) { return 120, 40, true }}
		if err := cmd.run(context.Background(), append([]string{"nemosh"}, args...)); err != nil {
			t.Fatalf("nemosh %q: %v; stderr %q", args, err, stderr.String())
		}
		return stdout.String()
	}

	session := run("echo \"$COLUMNS $LINES\"\nshopt -u checkwinsize\nCOLUMNS=7\necho \"$COLUMNS\"\nshopt -s checkwinsize\necho \"$COLUMNS\"\n", "--norc", "-i")
	script := run("", "-c", `echo "${COLUMNS-unset} ${LINES-unset}"`)

	if session != "120 40\n7\n120\n" || script != "unset unset\n" {
		t.Fatalf("session %q, script %q; want 120 40, 7, 120, and unset unset", session, script)
	}
}

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
