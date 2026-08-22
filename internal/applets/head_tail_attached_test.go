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

// Attached option values, and the file headers that go with them.
//
// `head -n2` was refused while `head -2` and `head -n 2` both worked, which is
// the worst shape a gap can take: the user cannot predict which spelling the
// shell has. The cause was two option parsers -- parseAppletOptions takes an
// attached value (`-m755`) and streamOptionsAndOperands matches whole strings
// against a whitelist -- and head and tail used the second.
//
// Every expectation was measured against busybox-w32 v1.38.0 on 2026-08-22.

func runHeadTail(t *testing.T, dir, applet string, args ...string) (string, string, error) {
	t.Helper()
	found, ok := applets.DefaultRegistry.Lookup(applet)
	if !ok {
		t.Fatalf("%s is not registered", applet)
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := found.Run(ctx, args, strings.NewReader("s1\ns2\ns3\n"), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func headTailFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "n.txt"), []byte("1\n2\n3\n4\n5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n2\n3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x\ny\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHeadTail_attachedOptionValues(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "head", args: []string{"-n2", "n.txt"}, want: "1\n2\n"},
		{applet: "head", args: []string{"-n", "2", "n.txt"}, want: "1\n2\n"},
		{applet: "head", args: []string{"-2", "n.txt"}, want: "1\n2\n"},
		// Two bytes of "1\n2\n..." is "1\n", so -c counts bytes and not runes.
		{applet: "head", args: []string{"-c2", "n.txt"}, want: "1\n"},
		{applet: "tail", args: []string{"-n2", "n.txt"}, want: "4\n5\n"},
		{applet: "tail", args: []string{"-2", "n.txt"}, want: "4\n5\n"},
		// The sign is part of the request: + counts from the start.
		{applet: "tail", args: []string{"-n+2", "n.txt"}, want: "2\n3\n4\n5\n"},
		{applet: "tail", args: []string{"-c+3", "n.txt"}, want: "2\n3\n4\n5\n"},
		{applet: "tail", args: []string{"-n-2", "n.txt"}, want: "4\n5\n"},
		// head's negative form is everything but the last N.
		{applet: "head", args: []string{"-n-2", "n.txt"}, want: "1\n2\n3\n"},
		{applet: "head", args: []string{"-n0", "n.txt"}, want: ""},
	} {
		t.Run(test.applet+" "+strings.Join(test.args, " "), func(t *testing.T) {
			// Given
			dir := headTailFixture(t)

			// When
			stdout, stderr, err := runHeadTail(t, dir, test.applet, test.args...)

			// Then
			if err != nil {
				t.Fatalf("%s %v: %v (stderr %q)", test.applet, test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%s %v = %q, want %q", test.applet, test.args, stdout, test.want)
			}
		})
	}
}

// An attached value that is not a number names the value, not the option. That
// is busybox's message -- `head: invalid number '2c'` -- and it is the useful
// one, because the option was recognised and the argument was the problem.
func TestHeadTail_refusesABadAttachedValue(t *testing.T) {
	for _, test := range []struct {
		applet   string
		args     []string
		wantWord string
	}{
		{applet: "head", args: []string{"-n2c", "n.txt"}, wantWord: "2c"},
		{applet: "head", args: []string{"-2n", "n.txt"}, wantWord: "2n"},
		{applet: "tail", args: []string{"-nx", "n.txt"}, wantWord: "x"},
		{applet: "head", args: []string{"-n"}, wantWord: "-n"},
		// Still an unimplemented option rather than a count.
		{applet: "head", args: []string{"-z", "n.txt"}, wantWord: "z"},
		{applet: "tail", args: []string{"-f", "n.txt"}, wantWord: "f"},
	} {
		t.Run(test.applet+" "+strings.Join(test.args, " "), func(t *testing.T) {
			// Given
			dir := headTailFixture(t)

			// When
			stdout, stderr, err := runHeadTail(t, dir, test.applet, test.args...)

			// Then
			if err == nil {
				t.Fatalf("%s %v succeeded, want a refusal", test.applet, test.args)
			}
			message := stderr + err.Error()
			if !strings.Contains(message, test.wantWord) {
				t.Fatalf("%s %v said %q, want it to name %q", test.applet, test.args, message, test.wantWord)
			}
			if strings.Contains(message, "No such file") || strings.Contains(message, "cannot open") {
				t.Fatalf("%s %v reports an option as a missing file: %q", test.applet, test.args, message)
			}
			if stdout != "" {
				t.Fatalf("%s %v wrote %q before refusing", test.applet, test.args, stdout)
			}
		})
	}
}

// More than one file operand gets a header naming each one, which is the whole
// reason `head *.log` is usable: without it the output is lines with no way to
// tell which file they came from. nemosh printed no headers at all before
// 2026-08-22, silently losing that.
func TestHeadTail_headsEachFileWhenThereIsMoreThanOne(t *testing.T) {
	dir := headTailFixture(t)

	stdout, stderr, err := runHeadTail(t, dir, "head", "-n1", "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("head: %v (%s)", err, stderr)
	}
	if want := "==> a.txt <==\n1\n\n==> b.txt <==\nx\n"; stdout != want {
		t.Fatalf("head two files gave %q, want %q", stdout, want)
	}

	stdout, _, err = runHeadTail(t, dir, "tail", "-n1", "a.txt", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "==> a.txt <==\n3\n\n==> b.txt <==\ny\n"; stdout != want {
		t.Fatalf("tail two files gave %q, want %q", stdout, want)
	}

	// The header names the operand as spelled, not a path this resolved.
	stdout, _, err = runHeadTail(t, dir, "head", "-n1", "./a.txt", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "==> ./a.txt <==") {
		t.Fatalf("head kept a resolved path in the header: %q", stdout)
	}
}

func TestHeadTail_oneFileHasNoHeader(t *testing.T) {
	dir := headTailFixture(t)
	for _, applet := range []string{"head", "tail"} {
		stdout, _, err := runHeadTail(t, dir, applet, "-n1", "a.txt")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stdout, "==>") {
			t.Fatalf("%s on one file printed a header: %q", applet, stdout)
		}
		// Nor does stdin, which has no name to print.
		stdout, _, err = runHeadTail(t, dir, applet, "-n1")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stdout, "==>") {
			t.Fatalf("%s on stdin printed a header: %q", applet, stdout)
		}
	}
}

// -q suppresses the headers and -v forces one, which is the pair that makes the
// default predictable rather than magic.
func TestHeadTail_quietAndVerbose(t *testing.T) {
	dir := headTailFixture(t)

	stdout, stderr, err := runHeadTail(t, dir, "head", "-q", "-n1", "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("head -q: %v (%s)", err, stderr)
	}
	if want := "1\nx\n"; stdout != want {
		t.Fatalf("head -q gave %q, want %q", stdout, want)
	}

	stdout, _, err = runHeadTail(t, dir, "head", "-v", "-n1", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "==> a.txt <==\n1\n"; stdout != want {
		t.Fatalf("head -v on one file gave %q, want %q", stdout, want)
	}

	stdout, _, err = runHeadTail(t, dir, "tail", "-q", "-n1", "a.txt", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "3\ny\n"; stdout != want {
		t.Fatalf("tail -q gave %q, want %q", stdout, want)
	}

	// Clustered with the count, since both are short options.
	stdout, _, err = runHeadTail(t, dir, "head", "-qn1", "a.txt", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\nx\n"; stdout != want {
		t.Fatalf("head -qn1 gave %q, want %q", stdout, want)
	}
}
