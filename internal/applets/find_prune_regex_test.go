package applets_test

import (
	"sort"
	"strings"
	"testing"
)

// -prune is true, and find does not go into a directory it is true of: `find . -name .git
// -prune -o -type f -print` lists the files outside .git. -regex matches the whole path as find
// prints it against a basic regular expression. Both were "unsupported expression". Each answer
// is busybox-w32's, measured on the same tree.
func TestFind_prunesAndMatchesARegex(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{
		"d/skip/a": "", "d/keep/b": "", "d/.git/c": "", "d/x.c": "", "d/y.h": "",
	})
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"d", "-name", "skip", "-prune", "-o", "-type", "f", "-print"}, "d/.git/c d/keep/b d/x.c d/y.h"},
		{[]string{"d", "-name", ".git", "-prune", "-o", "-type", "f", "-print"}, "d/keep/b d/skip/a d/x.c d/y.h"},
		{[]string{"d", "-prune"}, "d"},
		{[]string{"d", "-name", "skip", "-prune"}, "d/skip"},
		{[]string{"d", "-path", "d/skip", "-prune", "-o", "-print"}, "d d/.git d/.git/c d/keep d/keep/b d/x.c d/y.h"},
		{[]string{"d", "-regex", `.*\.[ch]`}, "d/x.c d/y.h"},
		{[]string{"d", "-regex", `d/[xy]\..*`}, "d/x.c d/y.h"},
		{[]string{"d", "-regex", "x.c"}, ""},
		{[]string{"d", "-regex", `.*/\(skip\|keep\)`}, "d/keep d/skip"},
	} {
		stdout, stderr, err := runFind(t, root, test.args...)
		lines := strings.Fields(stdout)
		sort.Strings(lines)
		if got := strings.Join(lines, " "); err != nil || got != test.want {
			t.Errorf("find %q = %q (stderr %q, err %v), want %q", test.args, got, stderr, err, test.want)
		}
	}
}
