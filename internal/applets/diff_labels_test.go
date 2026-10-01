package applets_test

import (
	"testing"
)

// -L names a file in the header, the first -L the first file and the last of the others the
// second; -T puts a tab after each line's marker, and -t writes a tab as the spaces to the next
// multiple of eight from the line's start. -q names the files whatever -L says. Each answer is
// busybox-w32's, measured; all three were taken and ignored.
func TestDiff_labelsAndTabsAsBusyboxDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"l": "a\tb\n", "r": "a\tc\n", "l8": "abcdefgh\tz\n", "r8": "abcdefgh\ty\n"})
	hunk := "@@ -1 +1 @@\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-L", "x", "-L", "y", "l", "r"}, want: "--- x\n+++ y\n" + hunk + "-a\tb\n+a\tc\n"},
		{args: []string{"-L", "x", "l", "r"}, want: "--- x\n+++ r\n" + hunk + "-a\tb\n+a\tc\n"},
		{args: []string{"-L", "x", "-L", "y", "-L", "z", "l", "r"}, want: "--- x\n+++ z\n" + hunk + "-a\tb\n+a\tc\n"},
		{args: []string{"-T", "l", "r"}, want: "--- l\n+++ r\n" + hunk + "-\ta\tb\n+\ta\tc\n"},
		{args: []string{"-t", "l", "r"}, want: "--- l\n+++ r\n" + hunk + "-a       b\n+a       c\n"},
		{args: []string{"-T", "-t", "l", "r"}, want: "--- l\n+++ r\n" + hunk + "-\ta       b\n+\ta       c\n"},
		{args: []string{"-t", "l8", "r8"}, want: "--- l8\n+++ r8\n" + hunk + "-abcdefgh        z\n+abcdefgh        y\n"},
		{args: []string{"-q", "-L", "x", "l", "r"}, want: "Files l and r differ\n"},
	} {
		if stdout, _, _ := runSmall(t, dir, "", "diff", test.args...); stdout != test.want {
			t.Errorf("diff %q = %q; want %q", test.args, stdout, test.want)
		}
	}
}
