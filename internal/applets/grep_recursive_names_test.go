package applets_test

import (
	"sort"
	"strings"
	"testing"
)

// grep -r names a file it found by the directory operand as written, then a slash unless the
// operand ends in one, as busybox's concat_path_file joins them: `grep -r x .` says ./t/f,
// where the join was cleaned and said t/f. A name appears when a directory was walked or more
// than one FILE was named, as busybox's grep_dir sets -H, so `grep -r x file` is the one file,
// unnamed. Each answer is busybox-w32's, measured.
func TestGrep_recursiveNamesAreTheOperandsPathsJoined(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"s/t/f": "needle\n", "top": "needle\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-r", "needle", "."}, "./s/t/f:needle\n./top:needle\n"},
		{[]string{"-r", "needle", "./"}, "./s/t/f:needle\n./top:needle\n"},
		{[]string{"-r", "needle", "./s"}, "./s/t/f:needle\n"},
		{[]string{"-r", "needle", "s/"}, "s/t/f:needle\n"},
		{[]string{"-r", "needle", "s//"}, "s//t/f:needle\n"},
		{[]string{"-rl", "needle", "."}, "./s/t/f\n./top\n"},
		{[]string{"-r", "needle", "top"}, "needle\n"},
		{[]string{"-r", "needle", "top", "s/t/f"}, "s/t/f:needle\ntop:needle\n"},
		{[]string{"-rH", "needle", "top"}, "top:needle\n"},
	} {
		stdout, stderr, err := runSmall(t, root, "", "grep", test.args...)
		lines := strings.SplitAfter(stdout, "\n")
		sort.Strings(lines)
		if got := strings.Join(lines, ""); err != nil || got != test.want {
			t.Errorf("grep %q = %q (stderr %q, err %v), want %q", test.args, got, stderr, err, test.want)
		}
	}
}
