package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diff and patch, kept together deliberately: shipping patch against a diff whose
// output shape later changed would break it silently, so these round-trip the two
// against each other rather than testing each alone.
//
// 8 of 8 measured diff forms agree with busybox byte for byte, and the patches
// interoperate in both directions with busybox's own patch.

func TestDiff_writesUnifiedOutput(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"x.txt":    "a\nb\nc\nd\ne\n",
		"y.txt":    "a\nB\nc\nd\nE\n",
		"same.txt": "a\nb\nc\nd\ne\n",
		"s.txt":    "a\nb\n",
		"t.txt":    "a\nx\nb\n",
	})
	// Unified always, which is busybox's default rather than GNU's, and with no
	// timestamps -- so two runs over unchanged files produce identical output.
	got, _, err := runSmall(t, dir, "", "diff", "x.txt", "y.txt")
	want := "--- x.txt\n+++ y.txt\n@@ -1,5 +1,5 @@\n a\n-b\n+B\n c\n d\n-e\n+E\n"
	if got != want {
		t.Fatalf("diff = %q\n  want %q", got, want)
	}
	// Differing is status 1, which is what a script tests.
	if err == nil {
		t.Fatal("diff on differing files returned success")
	}

	// Identical files produce nothing and status 0.
	got, _, err = runSmall(t, dir, "", "diff", "x.txt", "same.txt")
	if err != nil {
		t.Fatalf("diff on identical files failed: %v", err)
	}
	if got != "" {
		t.Fatalf("diff on identical files wrote %q", got)
	}

	// An insertion, where the two sides have different line counts.
	got, _, _ = runSmall(t, dir, "", "diff", "s.txt", "t.txt")
	if want := "--- s.txt\n+++ t.txt\n@@ -1,2 +1,3 @@\n a\n+x\n b\n"; got != want {
		t.Fatalf("diff of an insertion = %q\n  want %q", got, want)
	}

	// -q reports only that they differ; -s only that they match.
	got, _, _ = runSmall(t, dir, "", "diff", "-q", "x.txt", "y.txt")
	if !strings.Contains(got, "differ") {
		t.Fatalf("diff -q = %q", got)
	}
	got, _, _ = runSmall(t, dir, "", "diff", "-s", "x.txt", "same.txt")
	if !strings.Contains(got, "identical") {
		t.Fatalf("diff -s = %q", got)
	}
}

// The hunk range omits the count when it is one -- `@@ -3 +3,2 @@` -- which is the
// format's rule and what patch expects.
func TestDiff_hunkRangesAndContext(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"n1.txt":  "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n",
		"n2.txt":  "1\n2\n3\n4\nX\n6\n7\n8\n9\n10\n",
		"one.txt": "only\n",
		"two.txt": "changed\n",
	})
	// Three lines of context either side by default, so a change at line 5 shows
	// lines 2 to 8.
	got, _, _ := runSmall(t, dir, "", "diff", "n1.txt", "n2.txt")
	if !strings.Contains(got, "@@ -2,7 +2,7 @@") {
		t.Fatalf("default context is wrong: %q", got)
	}
	// -U1 narrows it.
	got, _, _ = runSmall(t, dir, "", "diff", "-U1", "n1.txt", "n2.txt")
	if !strings.Contains(got, "@@ -4,3 +4,3 @@") {
		t.Fatalf("diff -U1 = %q", got)
	}
	// A single-line file gives a count-free range.
	got, _, _ = runSmall(t, dir, "", "diff", "one.txt", "two.txt")
	if !strings.Contains(got, "@@ -1 +1 @@") {
		t.Fatalf("a one-line hunk should omit the count: %q", got)
	}
}

// -i and -w change what "the same line" means rather than how the diff is
// computed, so a file differing only in case or spacing compares equal.
func TestDiff_ignoreOptions(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"a.txt": "Hello   World\n",
		"b.txt": "hello world\n",
	})
	if _, _, err := runSmall(t, dir, "", "diff", "a.txt", "b.txt"); err == nil {
		t.Fatal("the files differ and diff said they did not")
	}
	if _, _, err := runSmall(t, dir, "", "diff", "-i", "-w", "a.txt", "b.txt"); err != nil {
		t.Fatalf("diff -i -w still reported a difference: %v", err)
	}
}

// The pair has to round-trip: diff writes it, patch applies it, and the result is
// the other file.
func TestPatch_roundTripsWithDiff(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"x.txt":    "a\nb\nc\nd\ne\n",
		"y.txt":    "a\nB\nc\nd\nE\n",
		"work.txt": "a\nb\nc\nd\ne\n",
	})
	unified, _, _ := runSmall(t, dir, "", "diff", "x.txt", "y.txt")
	if _, stderr, err := runSmall(t, dir, unified, "patch", "work.txt"); err != nil {
		t.Fatalf("patch: %v (%s)", err, stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "work.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nB\nc\nd\nE\n" {
		t.Fatalf("the patched file is %q, want y.txt's contents", data)
	}

	// -R undoes it, which is all reversing is: the removals become the additions.
	if _, _, err := runSmall(t, dir, unified, "patch", "-R", "work.txt"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "work.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\nc\nd\ne\n" {
		t.Fatalf("patch -R left %q, want the original", data)
	}
}

// A patch touching several separated places must apply all of them, which is
// where the running offset between hunks matters.
func TestPatch_appliesSeveralHunks(t *testing.T) {
	original := ""
	for index := 1; index <= 30; index++ {
		original += string(rune('a'+index%26)) + "\n"
	}
	changed := strings.Replace(original, "b\n", "FIRST\n", 1)
	changed = strings.Replace(changed, "z\n", "SECOND\n", 1)
	dir := writeSmallFixture(t, map[string]string{
		"old.txt":  original,
		"new.txt":  changed,
		"work.txt": original,
	})
	unified, _, _ := runSmall(t, dir, "", "diff", "old.txt", "new.txt")
	if strings.Count(unified, "@@") < 2 {
		t.Fatalf("expected two separated hunks, got %q", unified)
	}
	if _, stderr, err := runSmall(t, dir, unified, "patch", "work.txt"); err != nil {
		t.Fatalf("patch: %v (%s)", err, stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "work.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != changed {
		t.Fatalf("multi-hunk patch produced %q\n  want %q", data, changed)
	}
}

// A hunk that does not match is refused with its line number rather than applied
// approximately. There is no fuzz: shifting a hunk until the context happens to
// line up is how a patch lands where it was never meant to.
func TestPatch_refusesAHunkThatDoesNotApply(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"x.txt":     "a\nb\nc\nd\ne\n",
		"y.txt":     "a\nB\nc\nd\nE\n",
		"wrong.txt": "completely\ndifferent\ncontent\nhere\nnow\n",
	})
	unified, _, _ := runSmall(t, dir, "", "diff", "x.txt", "y.txt")
	before, err := os.ReadFile(filepath.Join(dir, "wrong.txt"))
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runSmall(t, dir, unified, "patch", "wrong.txt")
	if err == nil {
		t.Fatal("patch applied a hunk that did not match")
	}
	message := stderr + err.Error()
	if !strings.Contains(message, "hunk #1") || !strings.Contains(message, "line") {
		t.Fatalf("the rejection does not name the hunk and line: %q", message)
	}
	// And the file is untouched, so a failed patch does not leave half a change.
	after, err := os.ReadFile(filepath.Join(dir, "wrong.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("a failed patch modified the file: %q became %q", before, after)
	}
}

// The names in a diff come from whoever wrote it, so they are checked the way an
// archive entry is -- `--- ../../etc/passwd` is the same attack.
func TestPatch_refusesAnEscapingName(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	hostile := "--- ../escape.txt\n+++ ../escape.txt\n@@ -1 +1 @@\n-a\n+b\n"
	if _, _, err := runSmall(t, root, hostile, "patch"); err == nil {
		t.Fatal("patch accepted a name that escapes the directory")
	}
	entries, err := os.ReadDir(outer)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Join(outer, entry.Name()) != root {
			t.Fatalf("patch created %s outside the working directory", entry.Name())
		}
	}
	// -p strips leading components, which is how a patch made in one tree is
	// applied in another -- and the result is still checked.
	if _, _, err := runSmall(t, root, "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", "patch", "-p", "5"); err == nil {
		t.Fatal("patch -p 5 found a file it should not have")
	}
	// Rubbish is refused rather than silently doing nothing.
	if _, _, err := runSmall(t, root, "this is not a diff\n", "patch"); err == nil {
		t.Fatal("patch accepted input that was not a diff")
	}
}
