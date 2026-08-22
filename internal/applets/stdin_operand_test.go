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

// A lone `-` names standard input.
//
// POSIX gives it that meaning for every utility taking file operands, and it is
// how a script mixes a stream into a list of files:
// `cat header.txt - footer.txt`. Eleven applets answered
// `No such file or directory` instead, measured against busybox-w32 v1.38.0 on
// 2026-08-22 -- while cut, uniq, paste and comm each had their own check for it.

// The applets that take file operands and read them as text. Each is given `-`
// with something on stdin, and must read the stdin.
func TestStdinOperand_isReadForEveryApplet(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
		stdin  string
		want   string
	}{
		{applet: "cat", args: []string{"-"}, stdin: "a\nb\n", want: "a\nb\n"},
		{applet: "head", args: []string{"-n1", "-"}, stdin: "a\nb\n", want: "a\n"},
		{applet: "tail", args: []string{"-n1", "-"}, stdin: "a\nb\n", want: "b\n"},
		// No padding: busybox writes `2 -` for a single count, and so does this.
		{applet: "wc", args: []string{"-l", "-"}, stdin: "a\nb\n", want: "2 -\n"},
		{applet: "grep", args: []string{"a", "-"}, stdin: "a\nb\n", want: "a\n"},
		{applet: "sed", args: []string{"-n", "1p", "-"}, stdin: "a\nb\n", want: "a\n"},
		{applet: "sort", args: []string{"-"}, stdin: "b\na\n", want: "a\nb\n"},
		{applet: "nl", args: []string{"-"}, stdin: "a\n", want: "     1\ta\n"},
		{applet: "rev", args: []string{"-"}, stdin: "ab\n", want: "ba\n"},
		{applet: "base64", args: []string{"-"}, stdin: "a\n", want: "YQo=\n"},
		{applet: "cut", args: []string{"-c1", "-"}, stdin: "ab\n", want: "a\n"},
		{applet: "uniq", args: []string{"-"}, stdin: "a\na\n", want: "a\n"},
	} {
		t.Run(test.applet, func(t *testing.T) {
			// Given
			applet, ok := applets.DefaultRegistry.Lookup(test.applet)
			if !ok {
				t.Fatalf("%s is not registered", test.applet)
			}
			var stdout, stderr bytes.Buffer
			ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})

			// When
			err := applet.Run(ctx, test.args, strings.NewReader(test.stdin), &stdout, &stderr)

			// Then
			if err != nil {
				t.Fatalf("%s %v: %v (stderr %q)", test.applet, test.args, err, stderr.String())
			}
			if stdout.String() != test.want {
				t.Fatalf("%s %v = %q, want %q", test.applet, test.args, stdout.String(), test.want)
			}
		})
	}
}

// md5sum names the operand in its output, so it gets its own case: the digest of
// the stdin, labelled `-`.
func TestStdinOperand_checksumNamesTheDash(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("md5sum")
	if !ok {
		t.Fatal("md5sum is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})
	if err := applet.Run(ctx, []string{"-"}, strings.NewReader("a\n"), &stdout, &stderr); err != nil {
		t.Fatalf("md5sum -: %v (%s)", err, stderr.String())
	}
	// The digest of "a\n", which busybox reports for the same input.
	if want := "60b725f10c9c85c70d97880dfe8191b3  -\n"; stdout.String() != want {
		t.Fatalf("md5sum - = %q, want %q", stdout.String(), want)
	}
}

// The point of `-` is mixing, so a stream between two files has to land in the
// right place.
func TestStdinOperand_mixesWithFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"head.txt": "first\n", "foot.txt": "last\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	applet, ok := applets.DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("cat is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, []string{"head.txt", "-", "foot.txt"}, strings.NewReader("middle\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("cat head.txt - foot.txt: %v (%s)", err, stderr.String())
	}
	if want := "first\nmiddle\nlast\n"; stdout.String() != want {
		t.Fatalf("cat with a dash in the middle = %q, want %q", stdout.String(), want)
	}
}

// Closing the operand must not close the shell's stdin, or the second `-` would
// read from a closed stream. Reading it twice yields the rest, not an error.
func TestStdinOperand_doesNotCloseTheSharedStdin(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("cat is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})
	if err := applet.Run(ctx, []string{"-", "-"}, strings.NewReader("only\n"), &stdout, &stderr); err != nil {
		t.Fatalf("cat - -: %v (%s)", err, stderr.String())
	}
	// The first operand consumed the stream; the second finds it empty rather
	// than closed.
	if want := "only\n"; stdout.String() != want {
		t.Fatalf("cat - - = %q, want %q", stdout.String(), want)
	}
}
