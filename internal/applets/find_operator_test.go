package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Operators, and the reason they are worth having: without them `find` is a
// single-predicate filter rather than find.
//
// Every expected value below was measured against busybox-w32 v1.38.0 in a tree
// of the same shape on 2026-08-22, except where a comment says otherwise --
// and the exceptions are all cases where busybox accepts something malformed.

func findLines(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	stdout, stderr, err := runFind(t, dir, args...)
	if err != nil {
		t.Fatalf("find %v: %v (stderr %q)", args, err, stderr)
	}
	trimmed := strings.TrimSuffix(stdout, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestFind_operators(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "or takes either side",
			args: []string{".", "-name", "*.txt", "-o", "-name", "*.md"},
			want: []string{"./a.txt", "./notes.md", "./sub/b.txt"},
		},
		{
			name: "-or is the long spelling of -o",
			args: []string{".", "-name", "*.txt", "-or", "-name", "*.md"},
			want: []string{"./a.txt", "./notes.md", "./sub/b.txt"},
		},
		{
			name: "bang negates",
			args: []string{".", "!", "-name", "*.txt"},
			want: []string{".", "./notes.md", "./sub", "./sub/keep.note"},
		},
		{
			name: "-not is the long spelling of bang",
			args: []string{".", "-not", "-name", "*.txt"},
			want: []string{".", "./notes.md", "./sub", "./sub/keep.note"},
		},
		{
			name: "an explicit -a is the implicit and",
			args: []string{".", "-type", "f", "-a", "-name", "*.txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "-and is the long spelling of -a",
			args: []string{".", "-type", "f", "-and", "-name", "*.txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			// The precedence case, chosen so the two readings differ: and binds
			// tighter, so this is `md OR (txt AND d)` and answers notes.md. Were
			// -o to bind tighter it would be `(md OR txt) AND d` and answer
			// nothing at all.
			name: "and binds tighter than or",
			args: []string{".", "-name", "*.md", "-o", "-name", "*.txt", "-a", "-type", "d"},
			want: []string{"./notes.md"},
		},
		{
			name: "parentheses override the precedence",
			args: []string{".", "(", "-name", "*.md", "-o", "-name", "*.txt", ")", "-a", "-type", "d"},
			want: nil,
		},
		{
			name: "parentheses group an or",
			args: []string{".", "(", "-name", "*.md", "-o", "-name", "*.txt", ")"},
			want: []string{"./a.txt", "./notes.md", "./sub/b.txt"},
		},
		{
			name: "bang applies to a parenthesised group",
			args: []string{".", "!", "(", "-name", "*.txt", "-o", "-name", "*.md", ")"},
			want: []string{".", "./sub", "./sub/keep.note"},
		},
		{
			name: "bang binds tighter than and",
			args: []string{".", "!", "-type", "d", "-a", "-name", "*.txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "nested parentheses",
			args: []string{".", "(", "(", "-name", "a.txt", ")", "-o", "(", "-name", "notes.md", ")", ")"},
			want: []string{"./a.txt", "./notes.md"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			got := findLines(t, dir, test.args...)

			// Then
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("find %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

func TestFind_depthOptions(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "maxdepth 0 is the operand alone",
			args: []string{".", "-maxdepth", "0"},
			want: []string{"."},
		},
		{
			name: "maxdepth 1 stops before the subdirectory contents",
			args: []string{".", "-maxdepth", "1"},
			want: []string{".", "./a.txt", "./notes.md", "./sub"},
		},
		{
			name: "mindepth 1 drops the operand itself",
			args: []string{".", "-mindepth", "1"},
			want: []string{"./a.txt", "./notes.md", "./sub", "./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "one level exactly",
			args: []string{".", "-mindepth", "1", "-maxdepth", "1"},
			want: []string{"./a.txt", "./notes.md", "./sub"},
		},
		{
			name: "mindepth 2 is the subdirectory contents",
			args: []string{".", "-mindepth", "2"},
			want: []string{"./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "a depth option combines with a predicate",
			args: []string{".", "-maxdepth", "1", "-name", "*.txt"},
			want: []string{"./a.txt"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			got := findLines(t, dir, test.args...)

			// Then
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("find %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

func TestFind_namePredicates(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "iname ignores case",
			args: []string{".", "-iname", "*.TXT"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "iname folds the subject as well as the pattern",
			args: []string{".", "-iname", "A.TxT"},
			want: []string{"./a.txt"},
		},
		{
			// -path matches the whole path as printed, so its wildcards cross
			// separators where -name's cannot.
			name: "path matches the whole path",
			args: []string{".", "-path", "./sub*"},
			want: []string{"./sub", "./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "path does not match a bare basename",
			args: []string{".", "-path", "b.txt"},
			want: nil,
		},
		{
			name: "ipath is path with the case folded",
			args: []string{".", "-ipath", "./SUB/*"},
			want: []string{"./sub/b.txt", "./sub/keep.note"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			got := findLines(t, dir, test.args...)

			// Then
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("find %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

// -size, on a tree with known byte counts: 0, 1, 100 and 3000 bytes.
//
// The default unit is 512-byte blocks, and the size is divided by the unit and
// **rounded up** before it is compared. That rounding is POSIX
// (`find`, -size: "the file size in bytes, divided by 512 and rounded up to the
// next integer") and it is what GNU find does for every unit.
//
// It is a deliberate divergence from busybox-w32, which compares the raw byte
// count against N*unit instead. Measured 2026-08-22 in a tree of this shape:
// `find . -size 1k` answers nothing under busybox, because no file is exactly
// 1024 bytes, where POSIX makes it every file of 1..1024 bytes. The reasoning is
// in docs/support-matrix.md; the short version is that busybox's reading makes
// an exact-match `-size` with a unit suffix almost unusable, and POSIX specifies
// the other one outright.
func TestFind_size(t *testing.T) {
	dir := t.TempDir()
	for name, size := range map[string]int{"empty": 0, "one": 1, "small": 100, "big": 3000} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "exact bytes", args: []string{".", "-size", "1c"}, want: []string{"./one"}},
		{name: "greater than", args: []string{".", "-size", "+100c"}, want: []string{"./big"}},
		// A directory reports size 0 here as it does under busybox, so the root
		// operand matches too -- the measured reference prints `.` for this.
		{name: "less than", args: []string{".", "-size", "-1c"}, want: []string{".", "./empty"}},
		{name: "kilobytes round up", args: []string{".", "-size", "1k"}, want: []string{"./one", "./small"}},
		{name: "blocks are the default unit", args: []string{".", "-size", "+1"}, want: []string{"./big"}},
		{name: "words are two bytes", args: []string{".", "-size", "50w"}, want: []string{"./small"}},
		// Every non-empty file here rounds up to exactly one megabyte, which is
		// the clearest demonstration that the rounding is real.
		{name: "megabytes", args: []string{".", "-size", "1M"}, want: []string{"./big", "./one", "./small"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := findLines(t, dir, test.args...)
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("find %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

func TestFind_emptyAndTime(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "hollow"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blank"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "full"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "old")
	if err := os.WriteFile(old, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	twoDaysAgo := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, twoDaysAgo, twoDaysAgo); err != nil {
		t.Fatal(err)
	}

	// -empty is true for a zero-length file and for a directory with no entries,
	// which is why the directory case needs a read rather than a stat.
	if got := findLines(t, dir, ".", "-empty"); strings.Join(got, "|") != "./blank|./hollow" {
		t.Fatalf("-empty = %v, want ./blank ./hollow", got)
	}
	if got := findLines(t, dir, ".", "-empty", "-type", "d"); strings.Join(got, "|") != "./hollow" {
		t.Fatalf("-empty -type d = %v, want ./hollow", got)
	}
	// -mtime counts whole 24-hour periods, so +1 is "at least two days old".
	if got := findLines(t, dir, ".", "-mtime", "+1"); strings.Join(got, "|") != "./old" {
		t.Fatalf("-mtime +1 = %v, want ./old", got)
	}
	if got := findLines(t, dir, ".", "-type", "f", "-mtime", "0"); strings.Join(got, "|") != "./blank|./full" {
		t.Fatalf("-mtime 0 = %v, want the two fresh files", got)
	}
	// -newer compares against the operand's own mtime.
	if got := findLines(t, dir, ".", "-newer", "old", "-type", "f"); strings.Join(got, "|") != "./blank|./full" {
		t.Fatalf("-newer old = %v, want the two fresh files", got)
	}
}

// -print0 is what `xargs -0` is for, and the pairing only works if the separator
// is a NUL with no trailing newline of its own.
func TestFind_print0(t *testing.T) {
	dir := findFixture(t)
	stdout, stderr, err := runFind(t, dir, ".", "-name", "*.txt", "-print0")
	if err != nil {
		t.Fatalf("find: %v (stderr %q)", err, stderr)
	}
	if want := "./a.txt\x00./sub/b.txt\x00"; stdout != want {
		t.Fatalf("-print0 wrote %q, want %q", stdout, want)
	}
	if strings.Contains(stdout, "\n") {
		t.Fatalf("-print0 wrote a newline: %q", stdout)
	}
}

// An action anywhere in the expression suppresses the implicit -print, which is
// the rule that makes `find . -name x -print` print once rather than twice.
func TestFind_anActionSuppressesTheImplicitPrint(t *testing.T) {
	dir := findFixture(t)
	stdout, _, err := runFind(t, dir, ".", "-name", "a.txt", "-o", "-name", "notes.md", "-print")
	if err != nil {
		t.Fatal(err)
	}
	// `a.txt OR (notes.md AND print)`: only notes.md reaches the action.
	if stdout != "./notes.md\n" {
		t.Fatalf("stdout = %q, want ./notes.md once", stdout)
	}
}

// Where busybox is lax, this is not. Each of these is accepted by busybox-w32 --
// measured -- and each accepts a malformed expression by treating an operator as
// a path, which produces the "No such file or directory" diagnostic that names
// the wrong cause. GNU find refuses all of them and so does this.
func TestFind_refusesAMalformedExpression(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantWord string
	}{
		{name: "an unpaired open paren", args: []string{".", "(", "-name", "*.txt"}, wantWord: "("},
		{name: "an unpaired close paren", args: []string{".", "-name", "*.txt", ")"}, wantWord: ")"},
		{name: "a close paren alone", args: []string{".", ")"}, wantWord: ")"},
		{name: "an empty group", args: []string{".", "(", ")"}, wantWord: ")"},
		{name: "a trailing or", args: []string{".", "-name", "*.txt", "-o"}, wantWord: "-o"},
		{name: "a leading or", args: []string{".", "-o", "-name", "*.txt"}, wantWord: "-o"},
		{name: "a trailing and", args: []string{".", "-name", "*.txt", "-a"}, wantWord: "-a"},
		{name: "a trailing bang", args: []string{".", "-name", "*.txt", "!"}, wantWord: "!"},
		{name: "maxdepth without a number", args: []string{".", "-maxdepth"}, wantWord: "-maxdepth"},
		{name: "maxdepth with a word", args: []string{".", "-maxdepth", "deep"}, wantWord: "deep"},
		{name: "maxdepth with a negative number", args: []string{".", "-maxdepth", "-1"}, wantWord: "-maxdepth"},
		{name: "size without an argument", args: []string{".", "-size"}, wantWord: "-size"},
		{name: "size with a bad unit", args: []string{".", "-size", "1z"}, wantWord: "1z"},
		{name: "mtime with a word", args: []string{".", "-mtime", "soon"}, wantWord: "soon"},
		{name: "newer without an operand", args: []string{".", "-newer"}, wantWord: "-newer"},
		{name: "newer naming a missing file", args: []string{".", "-newer", "nosuch"}, wantWord: "nosuch"},
		{name: "iname without a pattern", args: []string{".", "-iname"}, wantWord: "-iname"},
		{name: "an action this build does not have", args: []string{".", "-exec", "echo", "{}", ";"}, wantWord: "-exec"},
		{name: "delete, which is refused deliberately", args: []string{".", "-delete"}, wantWord: "-delete"},
		{name: "depth, whose traversal order is not implemented", args: []string{".", "-depth"}, wantWord: "-depth"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			stdout, stderr, err := runFind(t, dir, test.args...)

			// Then
			if err == nil {
				t.Fatalf("find %v succeeded, want a refusal", test.args)
			}
			// Nothing may reach stdout: a caller piping into `xargs rm` must not
			// receive paths from an expression that was never valid.
			if stdout != "" {
				t.Fatalf("find %v wrote %q before refusing", test.args, stdout)
			}
			message := stderr + err.Error()
			if !strings.Contains(message, test.wantWord) {
				t.Fatalf("find %v said %q, want it to name %q", test.args, message, test.wantWord)
			}
			if strings.Contains(message, "No such file or directory") && test.wantWord != "nosuch" {
				t.Fatalf("find %v blames a missing file for an expression: %q", test.args, message)
			}
		})
	}
}

// An operator is not a path. Path collection stops at the first operand that
// could begin an expression, which is why `find . ! -name x` negates instead of
// looking for a file called `!` -- the measured busybox behaviour, and the
// failure this codebase already refuses to ship for `cat -n f.txt`.
func TestFind_doesNotTakeAnOperatorAsAPath(t *testing.T) {
	dir := findFixture(t)
	got := findLines(t, dir, ".", "!", "-name", "*.txt")
	if len(got) == 0 {
		t.Fatal("bang was consumed as a path")
	}
	_, stderr, err := runFind(t, dir, ".", "!", "-name", "*.txt")
	if err != nil {
		t.Fatalf("unexpected failure: %v (%s)", err, stderr)
	}
	if strings.Contains(stderr, "!") {
		t.Fatalf("stderr mentions the operator as a path: %q", stderr)
	}
}
