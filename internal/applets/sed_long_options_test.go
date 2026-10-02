package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sed takes busybox's long options as getopt_long takes them: --expression and --file with
// their value after `=` or as the next argument, --in-place with its suffix after `=`, and any
// of them by a prefix that names it alone. Only --quiet, --silent, --regexp-extended and a bare
// --in-place were taken. Each answer is busybox-w32's, measured.
func TestSed_longOptions(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "banana\napple\n", "s.sed": "s/a/X/\n", "b.txt": "banana\napple\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--expression=s/a/b/", "a.txt"}, want: "bbnana\nbpple\n"},
		{args: []string{"--expression", "s/a/b/", "a.txt"}, want: "bbnana\nbpple\n"},
		{args: []string{"--expr=s/a/b/", "a.txt"}, want: "bbnana\nbpple\n"},
		{args: []string{"--file=s.sed", "a.txt"}, want: "bXnana\nXpple\n"},
		{args: []string{"--file", "s.sed", "a.txt"}, want: "bXnana\nXpple\n"},
		{args: []string{"--quiet", "-e", "1p", "a.txt"}, want: "banana\n"},
		{args: []string{"--sil", "-e", "1p", "a.txt"}, want: "banana\n"},
	} {
		if got, _, err := runSmall(t, dir, "", "sed", test.args...); got != test.want || err != nil {
			t.Errorf("sed %q = %q, %v; want %q", test.args, got, err, test.want)
		}
	}
	if _, _, err := runSmall(t, dir, "", "sed", "--in-place=.bak", "s/a/Q/", "b.txt"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"b.txt": "bQnana\nQpple\n", "b.txt.bak": "banana\napple\n"} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
			t.Errorf("after --in-place=.bak, %s = %q, %v; want %q", name, data, err, want)
		}
	}
	if _, stderr, err := runSmall(t, dir, "", "sed", "--posix", "p", "a.txt"); err == nil || !strings.Contains(stderr+err.Error(), "unknown option -- posix") {
		t.Errorf("sed --posix = %q, %v; want a refusal naming it", stderr, err)
	}
}
