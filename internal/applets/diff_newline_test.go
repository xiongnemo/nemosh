package applets_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A last line without a newline is another line than the same text with one, and is said to
// have none, so files that differ only there differ, as busybox's do; -w and -b take the
// newline for white space, as busybox's read_token does. Files that hold a NUL differ in a line,
// but under -a. Each answer is busybox-w32's, measured: the files were the same, and a binary
// pair was diffed line by line.
func TestDiff_saysALastLineHasNoNewline(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"e1": "a\nb\n", "e2": "a\nb", "e3": "a\nb", "e4": "a\nc\n",
		"bin1": "a\x00b\n", "bin2": "a\x00c\n",
	})
	for _, test := range []struct {
		args []string
		want string
		same bool
	}{
		{args: []string{"e1", "e2"}, want: "--- e1\n+++ e2\n@@ -1,2 +1,2 @@\n a\n-b\n+b\n\\ No newline at end of file\n"},
		{args: []string{"e3", "e4"}, want: "--- e3\n+++ e4\n@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+c\n"},
		{args: []string{"-w", "e1", "e2"}, same: true},
		{args: []string{"-b", "e1", "e2"}, same: true},
		{args: []string{"e2", "e3"}, same: true},
		{args: []string{"bin1", "bin2"}, want: "Files bin1 and bin2 differ\n"},
		{args: []string{"-a", "bin1", "bin2"}, want: "--- bin1\n+++ bin2\n@@ -1 +1 @@\n-a\x00b\n+a\x00c\n"},
		{args: []string{"bin1", "bin1"}, same: true},
	} {
		stdout, _, err := runSmall(t, dir, "", "diff", test.args...)
		if same := err == nil; stdout != test.want || same != test.same || !same && !errors.Is(err, applets.ErrExitFalse) {
			t.Errorf("diff %q = %q, %v; want %q, the same %v", test.args, stdout, err, test.want, test.same)
		}
	}
}

// What diff writes for a file without a last newline, patch puts back without one.
func TestDiff_roundTripsALastLineWithoutANewlineThroughPatch(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"old": "a\nb\n", "new": "a\nc", "work": "a\nb\n"})
	unified, _, _ := runSmall(t, dir, "", "diff", "old", "new")
	if _, stderr, err := runSmall(t, dir, unified, "patch", "work"); err != nil {
		t.Fatalf("patch: %v (%s)", err, stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "work")); string(data) != "a\nc" {
		t.Errorf("patch made %q of the diff %q; want a, c and no last newline", data, unified)
	}
}
