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

// -A, -B, -C, -e, -f and -L.
//
// Context lines are most of why anyone reaches for grep on a log, and all three
// were refused by name until 2026-08-22. Every expectation below was measured
// against busybox-w32 v1.38.0 on that date; busybox's own usage text names
// exactly this surface, `[-m N] [-A|B|C N] { PATTERN | -e PATTERN... | -f FILE... }`.

// grepFixture is a file whose matches are deliberately both adjacent and far
// apart, so the group separator is exercised in both directions:
//
//	1 l1   2 M1   3 l3   4 M2   5 l5   6 l6   7 l7   8 l8   9 M3
func grepFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "g.txt"), []byte("l1\nM1\nl3\nM2\nl5\nl6\nl7\nl8\nM3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "h.txt"), []byte("other\nM1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.txt"), []byte("nomatch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runGrepIn(t *testing.T, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("grep is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestGrep_contextLines(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "after context",
			args: []string{"-A1", "M", "g.txt"},
			// M1's trailing line is l3, which is immediately followed by M2, so
			// the first two groups merge and only the jump to M3 needs a rule.
			want: "M1\nl3\nM2\nl5\n--\nM3\n",
		},
		{
			name: "before context",
			args: []string{"-B1", "M", "g.txt"},
			want: "l1\nM1\nl3\nM2\n--\nl8\nM3\n",
		},
		{
			name: "both",
			args: []string{"-C1", "M", "g.txt"},
			want: "l1\nM1\nl3\nM2\nl5\n--\nl8\nM3\n",
		},
		{
			name: "-C is -A and -B together",
			args: []string{"-A1", "-B1", "M", "g.txt"},
			want: "l1\nM1\nl3\nM2\nl5\n--\nl8\nM3\n",
		},
		{
			name: "a context wide enough to merge everything prints no separator",
			args: []string{"-C4", "M", "g.txt"},
			want: "l1\nM1\nl3\nM2\nl5\nl6\nl7\nl8\nM3\n",
		},
		{
			// Measured: busybox prints no separator at all when no context was
			// asked for, even though the groups are not adjacent. GNU does print
			// one here; busybox is the reference this follows.
			name: "zero context prints no separator",
			args: []string{"-A0", "M", "g.txt"},
			want: "M1\nM2\nM3\n",
		},
		{
			name: "a detached value",
			args: []string{"-A", "1", "M", "g.txt"},
			want: "M1\nl3\nM2\nl5\n--\nM3\n",
		},
		{
			// With -n a matching line is separated from its number by a colon and
			// a context line by a dash, which is how the two are told apart.
			name: "line numbers mark context with a dash",
			args: []string{"-A1", "-n", "M", "g.txt"},
			want: "2:M1\n3-l3\n4:M2\n5-l5\n--\n9:M3\n",
		},
		{
			name: "context stops at the max count but still trails it",
			args: []string{"-A1", "-m1", "M", "g.txt"},
			want: "M1\nl3\n",
		},
		{
			// Context applies to -o as well, and a context line prints whole
			// while a match prints only the matched part. Measured.
			name: "only-matching keeps whole context lines",
			args: []string{"-o", "-A1", "M", "g.txt"},
			want: "M\nl3\nM\nl5\n--\nM\n",
		},
		{
			name: "count ignores context",
			args: []string{"-c", "-A1", "M", "g.txt"},
			want: "3\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := grepFixture(t)

			// When
			stdout, stderr, err := runGrepIn(t, dir, "", test.args...)

			// Then
			if err != nil {
				t.Fatalf("grep %v: %v (stderr %q)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("grep %v\n  got  %q\n  want %q", test.args, stdout, test.want)
			}
		})
	}
}

// The separator carries across files, and the filename prefix distinguishes a
// match from a context line the same way the line number does.
func TestGrep_contextAcrossFiles(t *testing.T) {
	dir := grepFixture(t)
	stdout, stderr, err := runGrepIn(t, dir, "", "-A1", "M", "g.txt", "h.txt")
	if err != nil {
		t.Fatalf("grep: %v (%s)", err, stderr)
	}
	want := "g.txt:M1\ng.txt-l3\ng.txt:M2\ng.txt-l5\n--\ng.txt:M3\n--\nh.txt:M1\n"
	if stdout != want {
		t.Fatalf("grep across files\n  got  %q\n  want %q", stdout, want)
	}

	// A file with no match contributes nothing, and no dangling separator.
	stdout, _, err = runGrepIn(t, dir, "", "-C1", "M", "g.txt", "z.txt")
	if err != nil {
		t.Fatal(err)
	}
	want = "g.txt-l1\ng.txt:M1\ng.txt-l3\ng.txt:M2\ng.txt-l5\n--\ng.txt-l8\ng.txt:M3\n"
	if stdout != want {
		t.Fatalf("grep with a non-matching file\n  got  %q\n  want %q", stdout, want)
	}
}

// -e names a pattern, so several can be given and a pattern beginning with a
// dash stops looking like an option.
func TestGrep_severalPatterns(t *testing.T) {
	dir := grepFixture(t)

	stdout, stderr, err := runGrepIn(t, dir, "", "-e", "M1", "-e", "M3", "g.txt")
	if err != nil {
		t.Fatalf("grep -e: %v (%s)", err, stderr)
	}
	if want := "M1\nM3\n"; stdout != want {
		t.Fatalf("grep -e -e = %q, want %q", stdout, want)
	}

	// With -e present the first operand is a file, not the pattern.
	stdout, _, err = runGrepIn(t, dir, "", "-e", "M1", "g.txt", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "g.txt:M1\nh.txt:M1\n"; stdout != want {
		t.Fatalf("grep -e over two files = %q, want %q", stdout, want)
	}

	// -F applies to each pattern separately, so a metacharacter in one is
	// literal rather than escaping the alternation this builds.
	if err := os.WriteFile(filepath.Join(dir, "dots.txt"), []byte("a.c\nabc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = runGrepIn(t, dir, "", "-F", "-e", "a.c", "dots.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a.c\n"; stdout != want {
		t.Fatalf("grep -F -e a.c = %q, want %q", stdout, want)
	}

	// -x anchors each pattern rather than the alternation as a whole.
	stdout, _, err = runGrepIn(t, dir, "", "-x", "-e", "M1", "-e", "l3", "g.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "M1\nl3\n"; stdout != want {
		t.Fatalf("grep -x with two patterns = %q, want %q", stdout, want)
	}
}

func TestGrep_patternFile(t *testing.T) {
	dir := grepFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "pat.txt"), []byte("M1\nM3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.pat"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runGrepIn(t, dir, "", "-f", "pat.txt", "g.txt")
	if err != nil {
		t.Fatalf("grep -f: %v (%s)", err, stderr)
	}
	if want := "M1\nM3\n"; stdout != want {
		t.Fatalf("grep -f = %q, want %q", stdout, want)
	}

	// An empty pattern file matches nothing and exits 1, which is the measured
	// reference answer rather than the everything an empty pattern would match.
	stdout, _, err = runGrepIn(t, dir, "", "-f", "empty.pat", "g.txt")
	if err == nil {
		t.Fatal("grep -f with an empty pattern file succeeded, want status 1")
	}
	if stdout != "" {
		t.Fatalf("grep -f with an empty pattern file wrote %q", stdout)
	}

	// A missing pattern file is an error naming it, not a pattern.
	_, stderr, err = runGrepIn(t, dir, "", "-f", "nosuch.pat", "g.txt")
	if err == nil {
		t.Fatal("grep -f with a missing file succeeded")
	}
	if !strings.Contains(stderr+err.Error(), "nosuch.pat") {
		t.Fatalf("grep -f missing file said %q, want it to name the file", stderr+err.Error())
	}
}

// -L is -l inverted: the files that did *not* match. It exits 0 when it listed
// something, which is the opposite of what the match status would say.
func TestGrep_filesWithoutMatches(t *testing.T) {
	dir := grepFixture(t)

	stdout, stderr, err := runGrepIn(t, dir, "", "-L", "M", "g.txt", "z.txt")
	if err != nil {
		t.Fatalf("grep -L: %v (%s)", err, stderr)
	}
	if want := "z.txt\n"; stdout != want {
		t.Fatalf("grep -L = %q, want %q", stdout, want)
	}

	stdout, _, err = runGrepIn(t, dir, "", "-L", "M", "z.txt")
	if err != nil {
		t.Fatalf("grep -L on one non-matching file: %v", err)
	}
	if want := "z.txt\n"; stdout != want {
		t.Fatalf("grep -L one file = %q, want %q", stdout, want)
	}

	// Every file matched, so there is nothing to list.
	stdout, _, _ = runGrepIn(t, dir, "", "-L", "M", "g.txt", "h.txt")
	if stdout != "" {
		t.Fatalf("grep -L with all files matching wrote %q", stdout)
	}
}

func TestGrep_refusesABadContextValue(t *testing.T) {
	dir := grepFixture(t)
	for _, test := range []struct {
		args     []string
		wantWord string
	}{
		{args: []string{"-A", "x", "M", "g.txt"}, wantWord: "x"},
		{args: []string{"-Ax", "M", "g.txt"}, wantWord: "x"},
		{args: []string{"-A", "-1", "M", "g.txt"}, wantWord: "-1"},
		{args: []string{"-A"}, wantWord: "A"},
		{args: []string{"-e"}, wantWord: "e"},
		{args: []string{"-f"}, wantWord: "f"},
		// Still refused: busybox's usage text does not list these either.
		{args: []string{"-z", "M", "g.txt"}, wantWord: "z"},
		{args: []string{"-2", "M", "g.txt"}, wantWord: "2"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			stdout, stderr, err := runGrepIn(t, dir, "", test.args...)
			if err == nil {
				t.Fatalf("grep %v succeeded, want a refusal", test.args)
			}
			if !strings.Contains(stderr+err.Error(), test.wantWord) {
				t.Fatalf("grep %v said %q, want it to name %q", test.args, stderr+err.Error(), test.wantWord)
			}
			if stdout != "" {
				t.Fatalf("grep %v wrote %q before refusing", test.args, stdout)
			}
		})
	}
}
