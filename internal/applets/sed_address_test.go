package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Addresses, -n, -e, -E, and the p/d/q commands.
//
// sed was an `s///` filter and nothing else: no -n, no addresses, no d, no p.
// `sed -n '5,10p'` and `sed '/x/d'` are the two idioms people actually type, and
// neither ran. Measured against busybox-w32 v1.38.0 on 2026-08-22.

func runSedIn(t *testing.T, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("sed")
	if !ok {
		t.Fatal("sed is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func sedFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("a1\nb2\nc3\nd4\ne5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("a1\na2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f2.txt"), []byte("b1\nb2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSed_addressesAndCommands(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "quiet with a line address", args: []string{"-n", "2p", "s.txt"}, want: "b2\n"},
		{name: "a line range", args: []string{"-n", "2,4p", "s.txt"}, want: "b2\nc3\nd4\n"},
		{name: "delete one line", args: []string{"2d", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "delete by pattern", args: []string{"/b/d", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "a pattern range", args: []string{"-n", "/b/,/d/p", "s.txt"}, want: "b2\nc3\nd4\n"},
		{name: "the last line", args: []string{"-n", "$p", "s.txt"}, want: "e5\n"},
		{name: "to the last line", args: []string{"2,$d", "s.txt"}, want: "a1\n"},
		{name: "a negated address", args: []string{"-n", "2!p", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "several scripts", args: []string{"-e", "1d", "-e", "3d", "s.txt"}, want: "b2\nd4\ne5\n"},
		{name: "an address on a substitution", args: []string{"1,2s/[0-9]/X/", "s.txt"}, want: "aX\nbX\nc3\nd4\ne5\n"},
		// Without -n, p prints a second copy: the line is written once because
		// it reached the end of the script and once because p asked.
		{name: "p without -n duplicates", args: []string{"p", "s.txt"}, want: "a1\na1\nb2\nb2\nc3\nc3\nd4\nd4\ne5\ne5\n"},
		{name: "semicolons separate commands", args: []string{"-n", "1p;3p", "s.txt"}, want: "a1\nc3\n"},
		{name: "two substitutions in one script", args: []string{"s/a/X/;s/1/Y/", "s.txt"}, want: "XY\nb2\nc3\nd4\ne5\n"},
		{name: "q stops after printing", args: []string{"2q", "s.txt"}, want: "a1\nb2\n"},
		{name: "extended regex with -E", args: []string{"-E", "s/(a|b)[0-9]/Z/", "s.txt"}, want: "Z\nZ\nc3\nd4\ne5\n"},
		{name: "-r is the same as -E", args: []string{"-r", "s/(a|b)[0-9]/Z/", "s.txt"}, want: "Z\nZ\nc3\nd4\ne5\n"},
		{name: "a newline separates commands too", args: []string{"-n", "1p\n3p", "s.txt"}, want: "a1\nc3\n"},
		{name: "an address range that never closes runs to the end", args: []string{"-n", "/c/,/nosuch/p", "s.txt"}, want: "c3\nd4\ne5\n"},
		// A range whose numeric end has already gone by is one line long. `$,1p`
		// opens on the last line and closes on it: measured, busybox answers e5.
		{name: "a range whose end has passed is one line", args: []string{"-n", "$,1p", "s.txt"}, want: "e5\n"},
		{name: "a pattern that matches nothing prints nothing", args: []string{"-n", "/unclosed/p", "s.txt"}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := sedFixture(t)

			// When
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)

			// Then
			if err != nil {
				t.Fatalf("sed %v: %v (stderr %q)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %v\n  got  %q\n  want %q", test.args, stdout, test.want)
			}
		})
	}
}

// Several files are ONE stream: line numbers run on across the boundary and `$`
// is the last line of the last file. Measured -- `sed -n '3p' f1.txt f2.txt`
// answers b1, the third line overall.
//
// This is why the per-file loop had to go. It did not matter while sed had no
// addresses, and would have been silently wrong the moment it did.
func TestSed_severalFilesAreOneStream(t *testing.T) {
	dir := sedFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-n", "$p", "f1.txt", "f2.txt"}, want: "b2\n"},
		{args: []string{"-n", "1p", "f1.txt", "f2.txt"}, want: "a1\n"},
		{args: []string{"-n", "3p", "f1.txt", "f2.txt"}, want: "b1\n"},
		{args: []string{"-n", "2,3p", "f1.txt", "f2.txt"}, want: "a2\nb1\n"},
	} {
		stdout, stderr, err := runSedIn(t, dir, "", test.args...)
		if err != nil {
			t.Fatalf("sed %v: %v (%s)", test.args, err, stderr)
		}
		if stdout != test.want {
			t.Fatalf("sed %v = %q, want %q", test.args, stdout, test.want)
		}
	}
}

func TestSed_readsStdinWithAddresses(t *testing.T) {
	dir := sedFixture(t)
	stdout, _, err := runSedIn(t, dir, "one\ntwo\nthree\n", "-n", "2p")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "two\n" {
		t.Fatalf("sed -n 2p from stdin = %q, want two", stdout)
	}
	// `$` on a stream still needs to know which line is last, which means one
	// line of lookahead rather than reading the whole input.
	stdout, _, err = runSedIn(t, dir, "one\ntwo\nthree\n", "-n", "$p")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "three\n" {
		t.Fatalf("sed -n $p from stdin = %q, want three", stdout)
	}
}

// What sed still cannot do is refused by name, so a script asking for it fails
// rather than quietly getting something else.
func TestSed_refusesWhatItCannotDo(t *testing.T) {
	for _, test := range []struct {
		args     []string
		wantWord string
	}{
		// -i has to choose an output encoding for the file it rewrites, which is
		// the same decision already deferred for sed's UTF-16 reading. Bundling
		// it into a convenience flag would bury that.
		{args: []string{"-i", "s/a/X/", "s.txt"}, wantWord: "i"},
		{args: []string{"-f", "script.sed", "s.txt"}, wantWord: "f"},
		{args: []string{"y/a/b/", "s.txt"}, wantWord: "y"},
		{args: []string{"1a\\text", "s.txt"}, wantWord: "a"},
		{args: []string{"-n", "1~2p", "s.txt"}, wantWord: "~"},
		{args: []string{"-n", "/a/{p}", "s.txt"}, wantWord: "{"},
		{args: []string{"-n", "1,2h", "s.txt"}, wantWord: "h"},
		// A word of nonsense is reported by its first unimplemented letter,
		// which is what busybox does too: it answers `unsupported command o`
		// here, `n` being a command it has and this does not.
		{args: []string{"nosuchcommand", "s.txt"}, wantWord: "n"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			dir := sedFixture(t)
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)
			if err == nil {
				t.Fatalf("sed %v succeeded, want a refusal", test.args)
			}
			if !strings.Contains(stderr+err.Error(), test.wantWord) {
				t.Fatalf("sed %v said %q, want it to name %q", test.args, stderr+err.Error(), test.wantWord)
			}
			if stdout != "" {
				t.Fatalf("sed %v wrote %q before refusing", test.args, stdout)
			}
		})
	}
}

// A bad address is refused before any line is read, for the same reason find
// validates its expression first: a caller acting on the output must not receive
// half an answer.
func TestSed_refusesABadAddress(t *testing.T) {
	dir := sedFixture(t)
	// `/unclosed/p` and `$,1p` were in this list and should not have been: the
	// first is a perfectly good script that happens to match nothing, and the
	// second is a range whose numeric end has already passed, which busybox
	// answers with the last line. Both are asserted as *working* above. Checked
	// against the reference rather than assumed.
	for _, script := range []string{"2,p", "/a", "1,2,3p"} {
		stdout, stderr, err := runSedIn(t, dir, "", "-n", script, "s.txt")
		if err == nil {
			t.Fatalf("sed -n %q succeeded, want a refusal", script)
		}
		if stdout != "" {
			t.Fatalf("sed -n %q wrote %q before refusing", script, stdout)
		}
		if stderr+err.Error() == "" {
			t.Fatalf("sed -n %q refused with no message", script)
		}
	}
}
