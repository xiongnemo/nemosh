package applets_test

import (
	"regexp"
	"testing"
)

// ttysize prints the terminal's width and height, 80 24 without one, and each w or h asked for,
// as busybox's does. There was no ttysize.
func TestTtysize_printsTheSizeAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"ttysize"}, `^\d+ \d+\n$`},
		{[]string{"ttysize", "w"}, `^\d+\n$`},
		{[]string{"ttysize", "h", "w"}, `^\d+ \d+\n$`},
		{[]string{"ttysize", "x", "w"}, `^ \d+\n$`},
		{[]string{"ttysize", "-Z"}, `^\n$`},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		if err != nil || stderr != "" || !regexp.MustCompile(test.want).MatchString(stdout) {
			t.Errorf("%q: got %q, %q, %v; want %s", test.args, stdout, stderr, err, test.want)
		}
	}
}
