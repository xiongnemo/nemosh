package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Three files that were between 60% and 70%, and whose uncovered parts are all
// *tables*: `test`'s file-mode predicates, `echo -e`'s escape sequences, and
// winpath/posixpath's conversions. A table is exactly what a spot-check misses --
// one wrong row looks like every other row.

// Every -X predicate `test` accepts, against a real file, a real directory, and
// something that is not there. The last column is the one that matters: a predicate
// on a missing file must answer false rather than erroring, because
// `[ -f missing ]` is how a script asks.
func TestTest_fileModePredicates(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(root, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "adir")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "nope")

	for _, test := range []struct {
		operator string
		operand  string
		want     bool
	}{
		// -e is existence, which every other predicate implies.
		{operator: "-e", operand: file, want: true},
		{operator: "-e", operand: directory, want: true},
		{operator: "-e", operand: missing, want: false},
		// -f is a regular file, so a directory is not one.
		{operator: "-f", operand: file, want: true},
		{operator: "-f", operand: directory, want: false},
		{operator: "-f", operand: missing, want: false},
		// -d is the other way round.
		{operator: "-d", operand: directory, want: true},
		{operator: "-d", operand: file, want: false},
		{operator: "-d", operand: missing, want: false},
		// -s is "exists and is not empty", which is how a script checks a log.
		{operator: "-s", operand: file, want: true},
		{operator: "-s", operand: empty, want: false},
		{operator: "-s", operand: missing, want: false},
		// -r and -w read the permission bits.
		{operator: "-r", operand: file, want: true},
		{operator: "-w", operand: file, want: true},
		{operator: "-r", operand: missing, want: false},
		{operator: "-w", operand: missing, want: false},
		// The device and pipe predicates are false for an ordinary file rather than
		// erroring, which is what makes them usable in a condition.
		{operator: "-p", operand: file, want: false},
		{operator: "-S", operand: file, want: false},
		{operator: "-c", operand: file, want: false},
		{operator: "-b", operand: file, want: false},
		// The Unix mode bits Windows has no equivalent of: false rather than an
		// error, so a script that checks for a setuid binary gets an answer.
		{operator: "-u", operand: file, want: false},
		{operator: "-g", operand: file, want: false},
		{operator: "-k", operand: file, want: false},
		// -O and -G ask whether the effective user owns it. busybox-w32 answers them
		// from a stat that reports a fixed owner for everything, so the question
		// degrades to "does it exist" -- and this gives the same answer on every
		// platform rather than one that means something different on each.
		{operator: "-O", operand: file, want: true},
		{operator: "-G", operand: file, want: true},
		{operator: "-O", operand: missing, want: false},
	} {
		t.Run(test.operator+" "+filepath.Base(test.operand), func(t *testing.T) {
			// runTestApplet, not runSmall: it gives `test` no process view, so an
			// absolute operand resolves against the real working directory. The view
			// runSmall installs joins every path onto its cwd whether the path is
			// absolute or not, which turned each of these into nonsense.
			//
			// `test` reports through its exit status, which is what makes it usable in
			// a condition at all.
			status, stderr := runTestApplet(t, test.operator, test.operand)
			if got := status == 0; got != test.want {
				t.Fatalf("test %s %s exited %d, want %v (stderr %q)",
					test.operator, filepath.Base(test.operand), status, test.want, stderr)
			}
		})
	}
}

// A file with no read bit at all. Skipped where the platform cannot express it,
// which is honest rather than asserting a Windows behaviour as though it were the
// rule everywhere.
func TestTest_unreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skipf("this platform cannot remove every permission bit: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o444 != 0 {
		t.Skipf("chmod 000 left the mode at %v, so this platform does not track it", info.Mode().Perm())
	}
	if status, _ := runTestApplet(t, "-r", path); status == 0 {
		t.Fatal("test -r answered true for a file with no read bit")
	}
}

// The numeric and string comparisons, including what a non-numeric operand does:
// an error naming it rather than a silent zero, since `[ "$x" -eq 1 ]` with an
// empty x is a bug the shell should report.
func TestTest_comparisons(t *testing.T) {
	for _, test := range []struct {
		args []string
		want bool
	}{
		{args: []string{"1", "-eq", "1"}, want: true},
		{args: []string{"1", "-eq", "2"}, want: false},
		{args: []string{"1", "-ne", "2"}, want: true},
		{args: []string{"1", "-lt", "2"}, want: true},
		{args: []string{"2", "-lt", "1"}, want: false},
		{args: []string{"2", "-gt", "1"}, want: true},
		{args: []string{"1", "-le", "1"}, want: true},
		{args: []string{"1", "-ge", "1"}, want: true},
		// Negatives and a leading plus, both of which a bare Atoi would need to be
		// asked about explicitly.
		{args: []string{"-5", "-lt", "0"}, want: true},
		{args: []string{"+5", "-gt", "0"}, want: true},
		// Strings, where = and != are the operators and the comparison is byte-wise.
		{args: []string{"abc", "=", "abc"}, want: true},
		{args: []string{"abc", "=", "abd"}, want: false},
		{args: []string{"abc", "!=", "abd"}, want: true},
		// A lexical comparison is a different operator from a numeric one, so "10"
		// is less than "9" as a string and greater as a number.
		{args: []string{"10", "<", "9"}, want: true},
		{args: []string{"10", "-gt", "9"}, want: true},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			status, stderr := runTestApplet(t, test.args...)
			if got := status == 0; got != test.want {
				t.Fatalf("test %v exited %d, want %v (stderr %q)", test.args, status, test.want, stderr)
			}
		})
	}
	// A non-numeric operand to a numeric comparison is an error naming it.
	for _, args := range [][]string{{"abc", "-eq", "1"}, {"1", "-eq", "abc"}, {"", "-eq", "1"}} {
		// Exit 2, not 1: a bad expression is a different answer from a false one, and
		// conflating them is how `[ "$x" -eq 1 ]` with an empty x reads as "not equal"
		// rather than as the bug it is.
		if status, _ := runTestApplet(t, args...); status != 2 {
			t.Errorf("test %v exited %d, want 2 for an operand that is not a number", args, status)
		}
	}
}

// echo -e's escape table, every row of it. A wrong row here silently changes what a
// script writes, which is the failure mode a spot-check leaves in place.
func TestEcho_escapeSequences(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct{ input, want string }{
		{input: `a\ab`, want: "a\ab"},
		{input: `a\bb`, want: "a\bb"},
		{input: `a\eb`, want: "a\x1bb"},
		{input: `a\fb`, want: "a\fb"},
		{input: `a\nb`, want: "a\nb"},
		{input: `a\rb`, want: "a\rb"},
		{input: `a\tb`, want: "a\tb"},
		{input: `a\vb`, want: "a\vb"},
		{input: `a\\b`, want: `a\b`},
		// An octal escape, with and without the leading zero both references accept.
		{input: `a\101b`, want: "aAb"},
		{input: `a\0101b`, want: "aAb"},
		{input: `a\1b`, want: "a\x01b"},
		// A hex escape.
		{input: `a\x41b`, want: "aAb"},
		// Two hex digits are taken, so `a\x4b` is `a` and 0x4b and *not* a trailing
		// `b`. Measured: bash's `echo -ne "a\x4b" | od -c` answers `a  K`, which is
		// what settled these two after the first draft expected the letter as well.
		{input: `a\x4b`, want: "aK"},
		{input: `a\x0b`, want: "a\v"},
		// One hex digit followed by something that is not one stops at one digit.
		{input: `a\x4z`, want: "a\x04z"},
		// An escape that is not one is left as it was written, backslash and all --
		// which is what both references do rather than swallowing the backslash.
		{input: `a\qb`, want: `a\qb`},
		{input: `a\zb`, want: `a\zb`},
		// A trailing backslash with nothing after it.
		{input: `ab\`, want: `ab\`},
		// \c stops the output there and suppresses the newline, which is the one
		// escape that changes the *shape* of what is written rather than a byte.
		{input: `ab\cdef`, want: "ab"},
	} {
		t.Run(test.input, func(t *testing.T) {
			stdout, stderr, err := runSmall(t, root, "", "echo", "-e", "-n", test.input)
			if err != nil {
				t.Fatalf("echo -e: %v (%s)", err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("echo -e %q = %q, want %q", test.input, stdout, test.want)
			}
		})
	}
}

// Without -e a backslash is a backslash, which is what makes -e necessary rather
// than decorative.
func TestEcho_withoutDashEEscapesAreLiteral(t *testing.T) {
	root := t.TempDir()
	stdout, _, err := runSmall(t, root, "", "echo", "-n", `a\nb\tc`)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != `a\nb\tc` {
		t.Fatalf("echo without -e = %q, want the backslashes kept", stdout)
	}
	// And -E turns -e back off, which is how a script overrides a shell that
	// enables it by default.
	stdout, _, err = runSmall(t, root, "", "echo", "-e", "-E", "-n", `a\nb`)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != `a\nb` {
		t.Fatalf("echo -e -E = %q, want -E to win", stdout)
	}
}

// -n suppresses the trailing newline and nothing else; without it there is exactly
// one. Both halves matter to anything reading echo's output.
func TestEcho_theTrailingNewline(t *testing.T) {
	root := t.TempDir()
	stdout, _, err := runSmall(t, root, "", "echo", "one", "two")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "one two\n" {
		t.Fatalf("echo = %q, want the arguments joined by one space and a newline", stdout)
	}
	stdout, _, err = runSmall(t, root, "", "echo", "-n", "one", "two")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "one two" {
		t.Fatalf("echo -n = %q", stdout)
	}
	// No arguments is just the newline, not an error.
	stdout, _, err = runSmall(t, root, "", "echo")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "\n" {
		t.Fatalf("bare echo = %q, want one newline", stdout)
	}
}

// Something that looks like an option but is not one is an operand, because echo has
// no way to refuse: `echo -q` has to print `-q`. That is what makes echo unlike
// every other applet here, and it is worth an assertion rather than a comment.
func TestEcho_printsWhatLooksLikeAnUnknownOption(t *testing.T) {
	root := t.TempDir()
	for _, argument := range []string{"-q", "--help", "-", "--", "-en"} {
		stdout, _, err := runSmall(t, root, "", "echo", "-n", argument)
		if err != nil {
			t.Fatalf("echo -n %q: %v", argument, err)
		}
		// -en is a cluster echo does understand, so it prints nothing; the rest are
		// operands.
		if argument == "-en" {
			if stdout != "" {
				t.Errorf("echo -n -en printed %q", stdout)
			}
			continue
		}
		if stdout != argument {
			t.Errorf("echo -n %q printed %q", argument, stdout)
		}
	}
}
