package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -mmin -amin -cmin, -atime and -ctime, as busybox's find has them beside -mtime: the age in
// whole seconds against N minutes or days, +N past N+1 of them and -N short of N. A file
// stamped in the future is no -mtime 0, as in busybox, where it was. Each answer is
// busybox-w32's, measured; each but -mtime was refused as an unsupported expression.
func TestFind_timesInMinutesAndDays(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"new": "", "old": "", "future": ""})
	now := time.Now()
	old := now.Add(-3*time.Hour - 30*time.Second)
	if err := os.Chtimes(filepath.Join(dir, "old"), old, old); err != nil {
		t.Fatal(err)
	}
	future := now.Add(365 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "future"), future, future); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-type", "f", "-mmin", "-60"}, want: "./future|./new"},
		{args: []string{".", "-type", "f", "-mmin", "+60"}, want: "./old"},
		{args: []string{".", "-type", "f", "-mmin", "180"}, want: "./old"},
		{args: []string{".", "-type", "f", "-amin", "+60"}, want: "./old"},
		{args: []string{".", "-type", "f", "-atime", "0"}, want: "./new|./old"},
		{args: []string{".", "-type", "f", "-cmin", "-5"}, want: "./future|./new|./old"},
		{args: []string{".", "-type", "f", "-ctime", "+0"}, want: ""},
		{args: []string{".", "-name", "future", "-mtime", "0"}, want: ""},
		{args: []string{".", "-name", "future", "-mtime", "-1"}, want: "./future"},
	} {
		if got := findLines(t, dir, test.args...); strings.Join(got, "|") != test.want {
			t.Errorf("find %q = %v; want %q", test.args, got, test.want)
		}
	}
}
