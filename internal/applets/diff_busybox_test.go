package applets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What busybox-w32 1.38.0 printed for each of these, recorded from it, so that they hold where
// no busybox is installed, as on CI; diff_sweep_test.go compares with it where one is.

// runDiffOver writes files into a directory and diffs there, answering the output and status.
func runDiffOver(t *testing.T, files map[string]string, args ...string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errs bytes.Buffer
	err := newDiffApplet().Run(WithProcessView(context.Background(), hostProcessView{cwd: dir}), args, strings.NewReader(""), &out, &errs)
	switch {
	case err == nil:
		return out.String(), 0
	case errors.Is(err, ErrExitFalse):
		return out.String(), 1
	}
	t.Fatalf("diff %q: %v %s", args, err, errs.String())
	return "", 0
}

// stone tries at most max(256, √n) lines of a line's class, and -d every one. Here are two files
// of 2,401 lines drawn from three, where busybox's diff and its diff -d choose different edit
// scripts, 9,407 bytes and 9,386, too long to spell out, so they are checked by SHA-256.
func TestDiff_triesAsManyLinesOfAClassAsBusyboxDoes(t *testing.T) {
	state := uint64(1)
	draw := func() string {
		state = state*6364136223846793005 + 1442695040888963407
		return []string{"a\n", "b\n", "c\n"}[int(state>>33)%3]
	}
	var left, right strings.Builder
	left.WriteString("L\n")
	for range 2400 {
		left.WriteString(draw())
	}
	for range 2400 {
		right.WriteString(draw())
	}
	right.WriteString("R\n")
	files := map[string]string{"a": left.String(), "b": right.String()}
	for _, test := range []struct {
		args   []string
		length int
		sha256 string
	}{
		{[]string{"a", "b"}, 9407, "54ff7394892fc98dc5dcd8a2715dfd88774585e13cb920251fcc3fb693ce83f2"},
		{[]string{"-d", "a", "b"}, 9386, "d8e974fe62b7ead02abd28f1ebbc2cfdb4465bffd88ff9f5c25c6f41e074266e"},
	} {
		got, status := runDiffOver(t, files, test.args...)
		sum := sha256.Sum256([]byte(got))
		if status != 1 || len(got) != test.length || hex.EncodeToString(sum[:]) != test.sha256 {
			t.Errorf("diff %q: status %d, %d bytes, SHA-256 %x; busybox printed %d bytes, %s", test.args, status, len(got), sum, test.length, test.sha256)
		}
	}
}

// Lines are matched by busybox's hash of them, and a second reading unmatches two that share
// it and differ. `Aé` and `BE*` share it, 65·127² + 195·127 + 169 = 66·127² + 69·127 + 42 before
// the newline, so busybox takes the first lines for a common prefix, unmatches them, and
// matches nothing: the shared `BE*` is removed and added again, as it prints it.
func TestDiff_twoLinesThatShareAHashAreNoMatch(t *testing.T) {
	got, status := runDiffOver(t, map[string]string{"a": "Aé\nBE*\n", "b": "BE*\nZ\n"}, "a", "b")
	if want := "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-Aé\n-BE*\n+BE*\n+Z\n"; status != 1 || got != want {
		t.Errorf("status %d\n got %q\nwant %q", status, got, want)
	}
}

// -i lowers A to Z alone, as busybox's read_token does by hand: `Ä` and `ä` differ under it.
// strings.ToLower lowered them alike.
func TestDiff_ignoreCaseFoldsASCIIAlone(t *testing.T) {
	got, status := runDiffOver(t, map[string]string{"a": "Ä\nx\n", "b": "ä\nx\n"}, "-i", "a", "b")
	if want := "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-Ä\n+ä\n x\n"; status != 1 || got != want {
		t.Errorf("Ä against ä: status %d\n got %q\nwant %q", status, got, want)
	}
	if got, status := runDiffOver(t, map[string]string{"a": "A\nx\n", "b": "a\nx\n"}, "-i", "a", "b"); status != 0 || got != "" {
		t.Errorf("A against a: status %d, %q", status, got)
	}
}

// Under -U0 a hunk of additions or of removals alone is numbered, on the side it has no line of,
// by the line it follows. It was `-0,0`, which says before the first.
func TestDiff_aHunkWithNoLineOfASideIsNumberedByTheLineItFollows(t *testing.T) {
	files := map[string]string{"a": "a\nb\nc\n", "b": "a\nb\nX\nY\nc\n", "c": "a\nX\nb\nc\n"}
	if got, _ := runDiffOver(t, files, "-U0", "a", "b"); got != "--- a\n+++ b\n@@ -2,0 +3,2 @@\n+X\n+Y\n" {
		t.Errorf("added: %q", got)
	}
	if got, _ := runDiffOver(t, files, "-U0", "c", "a"); got != "--- c\n+++ a\n@@ -2 +1,0 @@\n-X\n" {
		t.Errorf("removed: %q", got)
	}
}
