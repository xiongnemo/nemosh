package applets_test

import (
	"strings"
	"testing"
)

// A symbolic link over a NAME that is there is "File exists", and one into a directory that is
// not there is "No such file or directory", as busybox says them. Both said "Permission denied"
// on Windows, where Go's Symlink tries again without the unprivileged flag after any failure
// and reports the second try's missing privilege. Each answer is busybox-w32's, measured.
func TestLn_symbolicLinkFailuresSayWhy(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "hi\n", "taken": ""})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-s", "a.txt", "taken"}, want: "taken: File exists"},
		{args: []string{"-s", "a.txt", "nod/x"}, want: "nod/x: No such file or directory"},
	} {
		if _, stderr, err := runSmall(t, dir, "", "ln", test.args...); err == nil || !strings.Contains(stderr+err.Error(), test.want) {
			t.Errorf("ln %q = %q, %v; want a failure saying %q", test.args, stderr, err, test.want)
		}
	}
}
