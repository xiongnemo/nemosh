package applets_test

import (
	"strings"
	"testing"
)

// -depth walks a directory's entries before the directory, as busybox's find does, and the
// bounds still bound it; -prune stops nothing under it. Each answer is busybox-w32's,
// measured; -depth was refused as an unsupported expression.
func TestFind_depthWalksEntriesFirst(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"d/e/f": "", "d/g": "", "x": ""})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-depth"}, want: "./d/e/f|./d/e|./d/g|./d|./x|."},
		{args: []string{"d", "-depth", "-name", "e"}, want: "d/e"},
		{args: []string{".", "-depth", "-maxdepth", "1"}, want: "./d|./x|."},
		{args: []string{".", "-depth", "-mindepth", "2"}, want: "./d/e/f|./d/e|./d/g"},
		{args: []string{".", "-depth", "-name", "d", "-prune"}, want: "./d"},
		{args: []string{".", "-depth", "-type", "d"}, want: "./d/e|./d|."},
	} {
		if got := findLines(t, dir, test.args...); strings.Join(got, "|") != test.want {
			t.Errorf("find %q = %v; want %q", test.args, got, test.want)
		}
	}
}
