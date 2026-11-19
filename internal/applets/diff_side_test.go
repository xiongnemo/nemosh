package applets_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// diff -y lays the files side by side as GNU's does, and busybox's cannot: the columns padded
// with tabs unless -t, the gutter `|` for a changed line, `<` and `>` for one on one side only,
// `\` or `/` where only one of two lines ends with a newline, and a line cut at the half width,
// counted in columns, so a wide character is two. -W sets the width, 130 by default;
// --suppress-common-lines leaves out what is the same, and --left-column prints it once, with
// `(`. Every answer is GNU diffutils 3.12's, measured. It was `unknown option -- y`.
func TestDiff_sideBySideAsGnuDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"l":     "same\nold line\ndeleted\nkeep\ntab\there\nlast",
		"r":     "same\nnew line\nkeep\nadded\ntab\there\nlast\n",
		"a3":    "1\n2\n3\n",
		"b3":    "1\nx\ny\nz\n3\n",
		"wide":  strings.Repeat("x", 80) + "\nshort\n",
		"wide2": strings.Repeat("y", 80) + "\nshort\n",
		"uni":   "café 中文\nb\n",
		"uni2":  "cafe\nb\n",
	})
	for _, test := range []struct {
		args   []string
		want   string
		status int
	}{
		{[]string{"-y", "l", "r"}, "same\t\t\t\t\t\t\t\tsame\nold line\t\t\t\t\t\t      |\tnew line\ndeleted\t\t\t\t\t\t\t      <\nkeep\t\t\t\t\t\t\t\tkeep\n\t\t\t\t\t\t\t      >\tadded\ntab\there\t\t\t\t\t\t\ttab\there\nlast\t\t\t\t\t\t\t      \\\tlast\n", 1},
		{[]string{"-y", "-W", "40", "l", "r"}, "same\t\t\tsame\nold line\t   |\tnew line\ndeleted\t\t   <\nkeep\t\t\tkeep\n\t\t   >\tadded\ntab\there\t\ttab\there\nlast\t\t   \\\tlast\n", 1},
		{[]string{"--side-by-side", "--width=40", "-t", "l", "r"}, "same                  same\nold line           |  new line\ndeleted            <\nkeep                  keep\n                   >  added\ntab     here          tab     here\nlast               \\  last\n", 1},
		{[]string{"-y", "--suppress-common-lines", "-W", "40", "l", "r"}, "old line\t   |\tnew line\ndeleted\t\t   <\n\t\t   >\tadded\nlast\t\t   \\\tlast\n", 1},
		{[]string{"-y", "--left-column", "-W", "40", "l", "r"}, "same\t\t   (\nold line\t   |\tnew line\ndeleted\t\t   <\nkeep\t\t   (\n\t\t   >\tadded\ntab\there\t   (\nlast\t\t   \\\tlast\n", 1},
		{[]string{"-y", "-W", "40", "a3", "b3"}, "1\t\t\t1\n2\t\t   |\tx\n\t\t   >\ty\n\t\t   >\tz\n3\t\t\t3\n", 1},
		{[]string{"-y", "-W", "40", "b3", "a3"}, "1\t\t\t1\nx\t\t   |\t2\ny\t\t   <\nz\t\t   <\n3\t\t\t3\n", 1},
		{[]string{"-y", "-W", "40", "a3", "a3"}, "1\t\t\t1\n2\t\t\t2\n3\t\t\t3\n", 0},
		{[]string{"-y", "-W", "30", "wide", "wide2"}, "xxxxxxxxxxxxx |\tyyyyyyyyyyyyy\nshort\t\tshort\n", 1},
		{[]string{"-y", "-W", "30", "uni", "uni2"}, "café 中文     |\tcafe\nb\t\tb\n", 1},
		{[]string{"-y", "-W", "9", "a3", "b3"}, "1\t1\n2   |\tx\n    >\ty\n    >\tz\n3\t3\n", 1},
		{[]string{"-y", "-W", "1", "a3", "b3"}, " \n|\n>\n>\n \n", 1},
		{[]string{"-y", "-q", "a3", "b3"}, "Files a3 and b3 differ\n", 1},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "diff", test.args...)
		status := 0
		if errors.Is(err, applets.ErrExitFalse) {
			status = 1
		} else if err != nil {
			status = -1
		}
		if stdout != test.want || status != test.status {
			t.Errorf("diff %q = %q, status %d (%v, %s); want %q, status %d", test.args, stdout, status, err, stderr, test.want, test.status)
		}
	}
	for _, args := range [][]string{{"-y", "-W", "x", "a3", "b3"}, {"-y", "-W", "0", "a3", "b3"}} {
		if _, _, err := runSmall(t, dir, "", "diff", args...); err == nil {
			t.Errorf("diff %q succeeded", args)
		}
	}
}

// --color is GNU's: a header bold, an @@ line cyan, a line removed red and one added green, each
// reset before its newline; under -y only the lines on one side are coloured, reset after the
// newline. Plain --color is auto, which is no colour into what is not a terminal. Measured from
// GNU diffutils 3.12, but for the header's timestamps, which this diff does not write.
func TestDiff_coloursAsGnuDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a3": "1\n2\n3\n", "b3": "1\nx\ny\nz\n3\n", "n1": "a", "n2": "b\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--color=always", "-U0", "a3", "b3"}, "\x1b[1m--- a3\x1b[0m\n\x1b[1m+++ b3\x1b[0m\n\x1b[36m@@ -2 +2,3 @@\x1b[0m\n\x1b[31m-2\x1b[0m\n\x1b[32m+x\x1b[0m\n\x1b[32m+y\x1b[0m\n\x1b[32m+z\x1b[0m\n"},
		{[]string{"--colour=always", "a3", "b3"}, "\x1b[1m--- a3\x1b[0m\n\x1b[1m+++ b3\x1b[0m\n\x1b[36m@@ -1,3 +1,5 @@\x1b[0m\n 1\n\x1b[31m-2\x1b[0m\n\x1b[32m+x\x1b[0m\n\x1b[32m+y\x1b[0m\n\x1b[32m+z\x1b[0m\n 3\n"},
		{[]string{"--color=always", "n1", "n2"}, "\x1b[1m--- n1\x1b[0m\n\x1b[1m+++ n2\x1b[0m\n\x1b[36m@@ -1 +1 @@\x1b[0m\n\x1b[31m-a\x1b[0m\n\\ No newline at end of file\n\x1b[32m+b\x1b[0m\n"},
		{[]string{"--color=always", "-y", "-W", "40", "a3", "b3"}, "1\t\t\t1\n2\t\t   |\tx\n\x1b[32m\t\t   >\ty\n\x1b[0m\x1b[32m\t\t   >\tz\n\x1b[0m3\t\t\t3\n"},
		{[]string{"--color=always", "-q", "a3", "b3"}, "Files a3 and b3 differ\n"},
		{[]string{"--color", "-U0", "a3", "b3"}, "--- a3\n+++ b3\n@@ -2 +2,3 @@\n-2\n+x\n+y\n+z\n"},
		{[]string{"--color=never", "-U0", "a3", "b3"}, "--- a3\n+++ b3\n@@ -2 +2,3 @@\n-2\n+x\n+y\n+z\n"},
		// After --, a FILE named --color is a FILE.
		{[]string{"-U0", "--", "a3", "b3"}, "--- a3\n+++ b3\n@@ -2 +2,3 @@\n-2\n+x\n+y\n+z\n"},
	} {
		if stdout, stderr, _ := runSmall(t, dir, "", "diff", test.args...); stdout != test.want {
			t.Errorf("diff %q = %q (%s); want %q", test.args, stdout, stderr, test.want)
		}
	}
	if _, _, err := runSmall(t, dir, "", "diff", "--color=purple", "a3", "b3"); err == nil {
		t.Error("diff --color=purple succeeded")
	}
}
