package applets_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// diffDirectoryFixture is busybox's measurement pair: a file the same in both, one that
// differs, one under a shared subdirectory that differs, a file and an empty directory only in
// each, and a name that is a directory in one and a file in the other.
func diffDirectoryFixture(t *testing.T) string {
	t.Helper()
	root := writeSmallFixture(t, map[string]string{
		"dd1/same": "a\n", "dd2/same": "a\n", "dd1/f": "x\n", "dd2/f": "y\n",
		"dd1/sub/s": "p\n", "dd2/sub/s": "q\n", "dd1/only1": "o\n", "dd2/only2": "o\n", "dd2/typ": "t\n",
	})
	for _, directory := range []string{"dd1/onlysub1", "dd2/onlysub2", "dd1/typ"} {
		if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(directory)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Two directories are compared name by name, as busybox's diffdir compares them, and each
// answer here is busybox-w32's, measured. diff refused a directory as a FILE, and -r was taken
// and ignored.
func TestDiff_comparesDirectoriesAsBusyboxDoes(t *testing.T) {
	root := diffDirectoryFixture(t)
	changeF := "--- dd1/f\n+++ dd2/f\n@@ -1 +1 @@\n-x\n+y\n"
	changeS := "--- dd1/sub/s\n+++ dd2/sub/s\n@@ -1 +1 @@\n-p\n+q\n"
	for _, test := range []struct {
		args []string
		want string
		same bool
	}{
		{args: []string{"dd1", "dd2"}, want: changeF + "Only in dd1: only1\nOnly in dd2: only2\nOnly in dd1: onlysub1\n" +
			"Only in dd2: onlysub2\nCommon subdirectories: dd1/sub and dd2/sub\n" +
			"File dd1/typ is a directory while file dd2/typ is a regular file\n"},
		{args: []string{"-r", "dd1", "dd2"}, want: changeF + "Only in dd1: only1\nOnly in dd2: only2\n" + changeS + "Only in dd2: typ\n"},
		{args: []string{"-rq", "dd1", "dd2"}, want: "Files dd1/f and dd2/f differ\nOnly in dd1: only1\nOnly in dd2: only2\n" +
			"Files dd1/sub/s and dd2/sub/s differ\nOnly in dd2: typ\n"},
		{args: []string{"-rN", "dd1", "dd2"}, want: changeF + "--- dd1/only1\n+++ /dev/null\n@@ -1 +0,0 @@\n-o\n" +
			"--- /dev/null\n+++ dd2/only2\n@@ -0,0 +1 @@\n+o\n" + changeS + "--- /dev/null\n+++ dd2/typ\n@@ -0,0 +1 @@\n+t\n"},
		{args: []string{"-S", "only2", "dd1", "dd2"}, want: "Only in dd2: only2\nOnly in dd1: onlysub1\nOnly in dd2: onlysub2\n" +
			"Common subdirectories: dd1/sub and dd2/sub\nFile dd1/typ is a directory while file dd2/typ is a regular file\n"},
		{args: []string{"dd1", "dd2/f"}, want: changeF},
		{args: []string{"dd1/", "dd2/f"}, want: changeF},
		{args: []string{"dd1/sub", "dd1/sub"}, same: true},
		{args: []string{"-s", "dd1/sub", "dd1/sub"}, want: "Files dd1/sub/s and dd1/sub/s are identical\n", same: true},
	} {
		stdout, _, err := runSmall(t, root, "", "diff", test.args...)
		if same := err == nil; stdout != test.want || same != test.same || !same && !errors.Is(err, applets.ErrExitFalse) {
			t.Errorf("diff %q = %q, %v; want %q, the same %v", test.args, stdout, err, test.want, test.same)
		}
	}
}

// A name that is a directory in one and a file in the other is said, and is no difference, as
// busybox counts it; nor are subdirectories in common.
func TestDiff_aDirectoryAgainstAFileIsSaidButNotCounted(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"q2/typ": "t\n", "q3/sub/a": "a\n", "q4/sub/a": "b\n"})
	if err := os.MkdirAll(filepath.Join(root, "q1", "typ"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"q1", "q2"}, want: "File q1/typ is a directory while file q2/typ is a regular file\n"},
		{args: []string{"q3", "q4"}, want: "Common subdirectories: q3/sub and q4/sub\n"},
	} {
		if stdout, _, err := runSmall(t, root, "", "diff", test.args...); stdout != test.want || err != nil {
			t.Errorf("diff %q = %q, %v; want %q and no difference", test.args, stdout, err, test.want)
		}
	}
}
