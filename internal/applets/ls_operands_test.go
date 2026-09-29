package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Given several operands ls lists the files first, together and sorted, and then each
// directory under its name, a blank line between the blocks, as busybox's does. Each operand
// was listed where it stood with no name above a directory's entries, so `ls d2 d1` ran the two
// together and `ls d1 f` set a beside f as though a had been named.
func TestLs_headsEachDirectoryAmongSeveralOperands(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"d1", "d2", "empty"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"d1/a", "d2/b", "f", "g"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	view := permuteTestView{cwd: dir}
	for _, test := range []struct {
		args       []string
		wantStdout string
		wantStderr string
	}{
		{[]string{"ls", "d2", "d1"}, "d1:\na\n\nd2:\nb\n", ""},
		{[]string{"ls", "d1", "g", "f"}, "f\ng\n\nd1:\na\n", ""},
		{[]string{"ls", "empty", "d1"}, "d1:\na\n\nempty:\n", ""},
		{[]string{"ls", "-r", "d1", "d2", "f"}, "f\n\nd2:\nb\n\nd1:\na\n", ""},
		{[]string{"ls", "-d", "d1", "f"}, "d1\nf\n", ""},
		{[]string{"ls", "-R", "d1"}, "d1:\na\n", ""},
		{[]string{"ls", "d1"}, "a\n", ""},
		{[]string{"ls", "nosuch", "d1"}, "d1:\na\n", "ls: nosuch: No such file or directory\n"},
	} {
		stdout, stderr, _ := runPermuted(t, view, "", test.args...)
		if stdout != test.wantStdout || stderr != test.wantStderr {
			t.Errorf("%q: got %q, %q; want %q, %q, as busybox lists them", test.args, stdout, stderr, test.wantStdout, test.wantStderr)
		}
	}
}
