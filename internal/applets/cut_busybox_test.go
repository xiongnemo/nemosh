package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// cut cuts as busybox's does, each case measured against busybox-w32: -D keeps the list's
// order, -O and --output-delimiter join what is printed, -d's first character splits and the
// whole of it joins, -F splits at an extended regular expression and keeps the delimiters
// inside a range, and -b's -O goes only between ranges that do not touch. A -d of a newline
// cuts lines, counted afresh in each FILE. It took -b -c -f -d -s alone.
func TestCut_cutsAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for name, content := range map[string]string{
		"tabs":   "a\tb\tc\td\none\ttwo\nno tabs here\n\n\t\tlead\ntrail\t\n",
		"colons": "a:b:c:d\n1:2\nnone\n::\n:x:\n",
		"spaces": "1 2  3   4    5\n  lead space\nsolo\n\nx y\n",
		"lines":  "line1\nline2\nline3\nline4\nline5\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-D", "-f", "3,1", "tabs"}, "c\ta\none\nno tabs here\n\nlead\t\ntrail\n"},
		{[]string{"-D", "-f", "2,1,2", "tabs"}, "b\ta\tb\ntwo\tone\ttwo\nno tabs here\n\n\t\t\n\ttrail\t\n"},
		{[]string{"-O", ":", "-f", "1,3", "tabs"}, "a:c\none\nno tabs here\n\n:lead\ntrail\n"},
		{[]string{"--output-delimiter=::", "-f", "1-", "tabs"}, "a::b::c::d\none::two\nno tabs here\n\n::::lead\ntrail::\n"},
		{[]string{"-d", ":x", "-f", "1,2", "colons"}, "a:xb\n1:x2\nnone\n:x\n:xx\n"},
		{[]string{"-d", ":", "-s", "-f", "1", "colons"}, "a\n1\n\n\n"},
		{[]string{"-F", "2", "spaces"}, "2\nlead\nsolo\n\ny\n"},
		{[]string{"-F", "2-4", "spaces"}, "2  3   4\nlead space\nsolo\n\ny\n"},
		{[]string{"-F", "2-", "-O", ":", "spaces"}, "2  3   4    5\nlead space\nsolo\n\ny\n"},
		{[]string{"-s", "-F", "2", "spaces"}, "2\nlead\ny\n"},
		{[]string{"-D", "-F", "3,1", "spaces"}, "3 1\nspace\nsolo\n\nx\n"},
		{[]string{"-b", "1,3", "-O", ":", "colons"}, "a:b\n1:2\nn:n\n:\n:::\n"},
		{[]string{"-b", "1-2,4-5", "-O", "|", "colons"}, "a:|:c\n1:\nno|e\n::\n:x\n"},
		{[]string{"-D", "-b", "3,1", "colons"}, "ba\n21\nnn\n:\n::\n"},
		{[]string{"-d", "\n", "-f", "2,4", "lines"}, "line2\nline4\n"},
		{[]string{"-d", "\n", "-f", "2-", "-O", ",", "lines"}, "line2,line3,line4,line5\n"},
		{[]string{"-d", "\n", "-f", "1,3", "lines", "tabs"}, "line1\nline3\na\tb\tc\td\nno tabs here\n"},
		// Where two ranges overlap, busybox walks them apart and prints what they share
		// twice: a line with no tab and then a tab, and an empty second field as the first.
		{[]string{"-f", "1,1", "tabs"}, "a\none\nno tabs here\n\n\ntrail\n"},
		{[]string{"-f", "1-2,2-3", "tabs"}, "a\tb\tc\none\ttwo\nno tabs here\n\n\t\tlead\ntrail\t\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"cut"}, test.args...)...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("cut %q: %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
}
