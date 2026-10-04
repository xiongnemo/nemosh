package applets_test

import (
	"strings"
	"testing"
)

// find says what it cannot read in busybox's words: `-name requires an argument`, `invalid
// argument 'z' to '-type'`, `invalid number 'x'` for -maxdepth, -mtime and -size, `can't stat
// 'f'` for -newer's FILE, as nemosh says can't, and `unpaired '('`, with busybox's single
// quotes. Each had words or quotes of its own. b, p and s, which busybox's -type knows, are
// still refused by name, this walk having nothing to classify them by. Measured against
// busybox-w32.
func TestFind_saysWhatItCannotReadInBusyboxsWords(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "x"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-name"}, want: "-name requires an argument"},
		{args: []string{".", "-type", "z"}, want: "invalid argument 'z' to '-type'"},
		{args: []string{".", "-type", "bp"}, want: "invalid argument 'bp' to '-type'"},
		{args: []string{".", "-maxdepth", "x"}, want: "invalid number 'x'"},
		{args: []string{".", "-mtime", "x"}, want: "invalid number 'x'"},
		{args: []string{".", "-size", "2x"}, want: "invalid number '2x'"},
		{args: []string{".", "-newer", "nofile"}, want: "cannot stat 'nofile': No such file or directory"},
		{args: []string{".", "(", "-name", "a.txt"}, want: "unpaired '('"},
		// What neither find takes is unrecognized.
		{args: []string{".", "-fstype", "x"}, want: "unrecognized: -fstype"},
		{args: []string{".", "-user", "x"}, want: "unrecognized: -user"},
		{args: []string{".", "-exec", "rm", "{}"}, want: "-exec requires an argument"},
		{args: []string{".", "-exec", "echo", "{}", "{}", "+"}, want: "only one '{}' allowed for -exec +"},
	} {
		_, _, err := runSmall(t, dir, "", "find", test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("find %q = %v; want %q", test.args, err, test.want)
		}
	}
	if _, _, err := runSmall(t, dir, "", "find", ".", "-type", "p"); err == nil || !strings.Contains(err.Error(), "unsupported type 'p'") {
		t.Errorf("find . -type p = %v; want it refused by name", err)
	}
}

// -wholename is busybox's other name for -path, and was refused. Measured against busybox-w32.
func TestFind_wholenameIsPath(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"d/a.txt": "x", "b.txt": "y"})
	stdout, _, err := runSmall(t, dir, "", "find", ".", "-wholename", "./d/*")
	if err != nil || stdout != "./d/a.txt\n" {
		t.Errorf("find . -wholename './d/*' = %q, %v; want ./d/a.txt", stdout, err)
	}
}
