package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -L follows every symbolic link and -H the PATHs alone, as busybox's find stats them: a link
// to a directory is walked as one, -type sees what a link points at, and a dangling link stays
// the link. -follow is -L, and a `--` after the options ends them. A link back to a directory
// above it is not gone into again. Each answer is busybox-w32's, measured, but the last, where
// busybox goes round until the path is too long; -H and -L were taken for the expression.
func TestFind_followsLinksUnderLAndH(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"d/f": ""})
	for target, name := range map[string]string{"d": "link", "nope": "dangling"} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("this environment makes no symbolic links: %v", err)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-name", "f"}, want: "./d/f"},
		{args: []string{"-L", ".", "-name", "f"}, want: "./d/f|./link/f"},
		{args: []string{"-L", ".", "-type", "l"}, want: "./dangling"},
		{args: []string{".", "-type", "l"}, want: "./dangling|./link"},
		{args: []string{"-H", "link", "-name", "f"}, want: "link/f"},
		{args: []string{"-H", ".", "-name", "f"}, want: "./d/f"},
		{args: []string{".", "-follow", "-name", "f"}, want: "./d/f|./link/f"},
		{args: []string{"-L", ".", "-type", "d"}, want: ".|./d|./link"},
		{args: []string{"-HL", ".", "-name", "f"}, want: "./d/f|./link/f"},
		{args: []string{"--", ".", "-name", "f"}, want: "./d/f"},
	} {
		if got := findLines(t, dir, test.args...); strings.Join(got, "|") != test.want {
			t.Errorf("find %q = %v; want %q", test.args, got, test.want)
		}
	}
	if err := os.Symlink("..", filepath.Join(dir, "d", "up")); err != nil {
		t.Fatal(err)
	}
	if got := findLines(t, dir, "-L", ".", "-name", "f"); strings.Join(got, "|") != "./d/f|./link/f" {
		t.Errorf("find -L over a loop = %v; want ./d/f and ./link/f once each", got)
	}
}
