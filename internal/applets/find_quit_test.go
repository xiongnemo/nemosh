package applets_test

import (
	"strings"
	"testing"
)

// -quit ends the walk where it stands, and is an action, so no -print is added for it: `find .
// -name 'j*' -print -quit` names the first j and stops, and without -print nothing is named.
// Each answer is busybox-w32's, measured; -quit was refused as an unsupported expression.
func TestFind_quitEndsTheWalk(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"j1.txt": "", "j2.txt": "", "k.txt": ""})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-name", "j*", "-print", "-quit"}, want: "./j1.txt"},
		{args: []string{".", "-name", "j*", "-quit"}, want: ""},
		{args: []string{".", "-quit", "-name", "x"}, want: ""},
	} {
		if got := findLines(t, dir, test.args...); strings.Join(got, "|") != test.want {
			t.Errorf("find %q = %v; want %q", test.args, got, test.want)
		}
	}
}
