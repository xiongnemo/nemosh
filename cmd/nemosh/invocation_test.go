package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runInvocation(t *testing.T, stdin string, args ...string) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: strings.NewReader(stdin), stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), append([]string{"nemosh"}, args...))
	return runResult{stdout: stdout.String(), stderr: stderr.String(), err: err, status: processExitStatus(err)}
}

// The shell's options on its own command line, as `set` would take them. Every one of
// these was "invalid option" and status 2 before a line ran; each answer is busybox-w32's,
// measured, apart from the order of the letters in `$-`.
func TestInvocation_takesShellOptionsBeforeTheScript(t *testing.T) {
	script := filepath.Join(t.TempDir(), "errexit.sh")
	if err := os.WriteFile(script, []byte("echo \"before $1\"\nfalse\necho after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		stdin  string
		args   []string
		stdout string
		status int
	}{
		// `#!/bin/sh -e` is launched as exactly this (planScriptLaunch).
		{name: "-e before a script", args: []string{"-e", script, "one"}, stdout: "before one\n", status: 1},
		{name: "a script after --", args: []string{"--", script, "two"}, stdout: "before two\nafter\n"},
		{name: "a script after -", args: []string{"-", script}, stdout: "before \nafter\n"},
		{name: "-o pipefail", args: []string{"-o", "pipefail", "-c", `false | true; echo "$? $-"`}, stdout: "1 c\n"},
		{name: "letters grouped with c", args: []string{"-ec", `echo "$- $0 $1"`, "name", "one"}, stdout: "ec name one\n"},
		{name: "-c before the other letters", args: []string{"-c", "-u", `echo "$-"; echo "$unset"`}, stdout: "uc\n", status: 2},
		{name: "+ turns one off", args: []string{"-e", "+e", "-c", "false; echo survived"}, stdout: "survived\n"},
		{name: "-s gives stdin its positionals", stdin: `echo "$- $1 $2"`, args: []string{"-s", "a", "b"}, stdout: "s a b\n"},
		{name: "stdin is s without asking", stdin: `echo "$-"`, stdout: "s\n"},
		{name: "a script file adds nothing", args: []string{"-f", script}, stdout: "before \nafter\n"},
		{name: "-n runs nothing", args: []string{"-n", "-c", "echo ran"}},
		{name: "-o noexec is -n", args: []string{"-o", "noexec", script}},
		{name: "-n reports a syntax error", args: []string{"-n", "-c", "echo ran; if"}, status: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runInvocation(t, test.stdin, test.args...)
			if result.stdout != test.stdout || result.status != test.status {
				t.Fatalf("nemosh %q = %d %q, want %d %q (stderr %q)", test.args, result.status, result.stdout, test.status, test.stdout, result.stderr)
			}
		})
	}
}

func TestInvocation_tracesWithX(t *testing.T) {
	result := runInvocation(t, "", "-x", "-c", "echo hi")
	if result.stdout != "hi\n" || !strings.Contains(result.stderr, "+ echo hi") {
		t.Fatalf("stdout %q, stderr %q, want the command traced", result.stdout, result.stderr)
	}
}

// Refusals keep `set`'s words and status. A letter the shell does not have points at
// --help; one it has and will not turn on says why instead.
func TestInvocation_refusesWhatSetRefuses(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		words []string
		hint  bool
	}{
		{name: "an unknown letter", args: []string{"-q", "-c", "echo ran"}, words: []string{"illegal option -q"}, hint: true},
		{name: "an unknown name", args: []string{"-o", "nosuch", "-c", "echo ran"}, words: []string{"illegal option -o nosuch"}, hint: true},
		{name: "an inert letter", args: []string{"-v", "-c", "echo ran"}, words: []string{"-v: not implemented"}},
		{name: "-c with nothing to run", args: []string{"-e", "-c"}, words: []string{"-c requires an argument"}},
		{name: "-i with a script", args: []string{"-i", "-c", "echo ran"}, words: []string{"-i reads commands"}},
		{name: "an unknown long option", args: []string{"-e", "--nosuch"}, words: []string{"invalid option --nosuch"}, hint: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runInvocation(t, "", test.args...)
			if result.status != 2 || result.stdout != "" {
				t.Fatalf("nemosh %q = %d %q, want 2 and nothing run", test.args, result.status, result.stdout)
			}
			for _, word := range test.words {
				if !strings.Contains(result.stderr, word) {
					t.Fatalf("stderr %q, want %q", result.stderr, word)
				}
			}
			if strings.Contains(result.stderr, "--help") != test.hint {
				t.Fatalf("stderr %q, want the --help hint: %v", result.stderr, test.hint)
			}
		})
	}
}

// -l reads $HOME/.profile before the command, as busybox's login shell does; `-l` was an
// invalid option. Only the profile's own line is checked, because /etc/profile is the
// machine's and may say anything.
func TestInvocation_loginReadsTheProfile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".profile"), []byte("PROFILED=yes\necho from-profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	result := runInvocation(t, "", "-l", "-c", `echo "$PROFILED $-"`)
	if !strings.Contains(result.stdout, "from-profile\nyes c\n") {
		t.Fatalf("stdout %q, want the profile run before the command (stderr %q)", result.stdout, result.stderr)
	}
}

// A session says it is one: `case $- in *i*)` is how a startup file tells, and $0 is the
// shell's name. `$-` had no i and $0 was empty.
func TestInvocation_sessionReportsItself(t *testing.T) {
	result := runInvocation(t, "echo \"[$-] [$0] [$1]\"\n", "-i", "-s", "one")
	if !strings.Contains(result.stdout, "[is] [nemosh] [one]\n") {
		t.Fatalf("stdout %q, want the session's $-, $0 and $1", result.stdout)
	}
}

// A startup file runs as part of the shell. Its EXIT trap waits for the shell's exit, and
// an `exit` in it ends the shell with that status. Through RunScript the trap ran as soon
// as the profile ended, and `exit 3` ended only the profile. Both answers are busybox's.
func TestInvocation_startupFileIsPartOfTheShell(t *testing.T) {
	for _, test := range []struct {
		name, profile string
		stdout        string
		missing       string
		status        int
	}{
		{name: "the EXIT trap waits", profile: "trap 'echo bye' EXIT\necho profiled\n", stdout: "profiled\nmain\nbye\n"},
		{name: "exit ends the shell", profile: "echo profiled\nexit 3\necho after\n", stdout: "profiled\n", missing: "main", status: 3},
		{name: "a last status is not a failure", profile: "false\n", stdout: "main\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, ".profile"), []byte(test.profile), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)

			result := runInvocation(t, "", "-l", "-c", "echo main")
			if !strings.HasSuffix(result.stdout, test.stdout) || result.status != test.status {
				t.Fatalf("stdout %q status %d, want it to end %q with %d (stderr %q)", result.stdout, result.status, test.stdout, test.status, result.stderr)
			}
			if test.missing != "" && strings.Contains(result.stdout, test.missing) {
				t.Fatalf("stdout %q, want nothing after the profile's exit", result.stdout)
			}
			if strings.Contains(result.stderr, ".profile") {
				t.Fatalf("stderr %q, want the profile not reported", result.stderr)
			}
		})
	}
}

func TestInvocation_exitInENVEndsTheSession(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "rc.sh")
	if err := os.WriteFile(rc, []byte("exit 4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV", filepath.ToSlash(rc))

	result := runInvocation(t, "echo reached\n", "-i")
	if result.status != 4 || strings.Contains(result.stdout, "reached") {
		t.Fatalf("status %d stdout %q, want 4 before any line ran", result.status, result.stdout)
	}
}

// ignoreeof refuses an end of input at the prompt with busybox's words, fifty times in a
// row and then leaves, as busybox does -- so a pipe that has ended cannot hold the shell.
func TestInvocation_ignoreeofRefusesEndOfInput(t *testing.T) {
	result := runInvocation(t, "set -o ignoreeof\necho hi\n", "-i")
	if result.status != 0 || !strings.Contains(result.stdout, "hi\n") {
		t.Fatalf("status %d stdout %q, want the session to run and leave", result.status, result.stdout)
	}
	if got := strings.Count(result.stderr, "Use \"exit\" to leave shell."); got != maxIgnoredEOFs {
		t.Fatalf("refused %d times, want %d", got, maxIgnoredEOFs)
	}
}
