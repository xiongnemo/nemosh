package main

import (
	"bytes"
	"strings"
	"testing"
)

// argv.py prints its arguments as Python 2 prints a list of byte strings, and 695 lines of
// the vendored cases expect that output byte for byte: single quotes unless a string holds
// one and no double quote, the quote and backslash escaped, tab, newline and return by
// name, and any other byte outside printable ASCII as \xNN. The wants were produced by
// Python's own repr of the same bytes.
func TestRun_argvPrintsAsPython2(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "[]\n"},
		{[]string{""}, "['']\n"},
		{[]string{"a", "b c"}, "['a', 'b c']\n"},
		{[]string{"it's"}, "[\"it's\"]\n"},
		{[]string{`say "hi"`}, "['say \"hi\"']\n"},
		{[]string{`both ' and "`}, `['both \' and "']` + "\n"},
		{[]string{`a\b`}, `['a\\b']` + "\n"},
		{[]string{"tab\there", "\n\r"}, `['tab\there', '\n\r']` + "\n"},
		{[]string{"\x01", "\x7f", "é"}, `['\x01', '\x7f', '\xc3\xa9']` + "\n"},
	}
	for _, test := range tests {
		stdout, _, status := runHelper(t, "argv.py", test.args, nil)
		if stdout != test.want || status != 0 {
			t.Errorf("argv.py %q printed %q and returned %d, want %q", test.args, stdout, status, test.want)
		}
	}
}

// printenv.py prints each variable it is asked for, and None for one that is not set.
// The lookup is exact, as it is on the Unix the cases were written on.
func TestRun_printenvPrintsNoneForAnUnsetName(t *testing.T) {
	stdout, _, status := runHelper(t, "printenv.py", []string{"FOO", "foo", "EMPTY", "BAR"}, []string{"FOO=x=y", "EMPTY="})

	if want := "x=y\nNone\n\nNone\n"; stdout != want || status != 0 {
		t.Errorf("printed %q and returned %d, want %q", stdout, status, want)
	}
}

func TestRun_stdoutStderrPrintsWhereItIsTold(t *testing.T) {
	tests := []struct {
		args           []string
		stdout, stderr string
		status         int
	}{
		{nil, "STDOUT\n", "STDERR\n", 0},
		{[]string{"out"}, "out\n", "STDERR\n", 0},
		{[]string{"out", "err", "3"}, "out\n", "err\n", 3},
	}
	for _, test := range tests {
		stdout, stderr, status := runHelper(t, "stdout_stderr.py", test.args, nil)
		if stdout != test.stdout || stderr != test.stderr || status != test.status {
			t.Errorf("stdout_stderr.py %q gave %q %q %d, want %q %q %d", test.args, stdout, stderr, status, test.stdout, test.stderr, test.status)
		}
	}
}

// Joined, the two streams come stderr first, as Python 2's do when stdout is not a
// terminal: it writes stderr at once and stdout at exit. `stdout_stderr.py |& cat`
// expects exactly that.
func TestRun_stdoutStderrWritesStderrFirst(t *testing.T) {
	var joined bytes.Buffer

	run("stdout_stderr.py", nil, nil, &joined, &joined)

	if want := "STDERR\nSTDOUT\n"; joined.String() != want {
		t.Errorf("joined output %q, want %q", joined.String(), want)
	}
}

// spec/bin/foo=bar is a script that prints HI, found on PATH by a case that runs foo\=bar.
func TestRun_fooEqualsBarSaysHi(t *testing.T) {
	if stdout, _, status := runHelper(t, "foo=bar", nil, nil); stdout != "HI\n" || status != 0 {
		t.Errorf("printed %q and returned %d", stdout, status)
	}
}

func TestRun_refusesANameItDoesNotStandFor(t *testing.T) {
	_, stderr, status := runHelper(t, "show_fd_table.py", nil, nil)

	if status != 2 || !strings.Contains(stderr, "show_fd_table.py") {
		t.Errorf("returned %d with %q, want 2 and the name", status, stderr)
	}
}

func runHelper(t *testing.T, name string, args, env []string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := run(name, args, env, &stdout, &stderr)
	return stdout.String(), stderr.String(), status
}
