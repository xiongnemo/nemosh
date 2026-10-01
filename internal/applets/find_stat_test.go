package applets_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// -perm, -links, -samefile, -inum and -executable, as busybox's find has them, asking what stat
// says: the mode stat shows, its hard links, and its volume and file index. Each answer is
// busybox-w32's, measured; each was refused as an unsupported expression. The modes that tell
// on the umask are left out, as that is a decision of its own.
func TestFind_statPredicates(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "", "b.exe": "", "c.sh": "", "d.sh": "#!/bin/sh\necho\n", "sub/x": ""})
	if err := os.Link(filepath.Join(dir, "a.txt"), filepath.Join(dir, "hard.txt")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		// Windows makes the x bits up from the name and the first line; elsewhere they are set.
		for _, name := range []string{"b.exe", "c.sh", "d.sh"} {
			if err := os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	inode, _, err := runSmall(t, dir, "", "stat", "-c", "%i", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{".", "-type", "f", "-perm", "-u+w"}, want: "./a.txt|./b.exe|./c.sh|./d.sh|./hard.txt|./sub/x"},
		{args: []string{".", "-perm", "/o+w"}, want: ""},
		{args: []string{".", "-type", "f", "-perm", "-111"}, want: "./b.exe|./c.sh|./d.sh"},
		{args: []string{".", "-type", "f", "-executable"}, want: "./b.exe|./c.sh|./d.sh"},
		{args: []string{".", "-type", "f", "-links", "2"}, want: "./a.txt|./hard.txt"},
		{args: []string{".", "-type", "f", "-links", "+1"}, want: "./a.txt|./hard.txt"},
		{args: []string{".", "-type", "f", "-links", "-2"}, want: "./b.exe|./c.sh|./d.sh|./sub/x"},
		{args: []string{".", "-samefile", "a.txt"}, want: "./a.txt|./hard.txt"},
		{args: []string{".", "-inum", strings.TrimSpace(inode)}, want: "./a.txt|./hard.txt"},
		{args: []string{".", "-inum", "0"}, want: ""},
	} {
		if got := findLines(t, dir, test.args...); strings.Join(got, "|") != test.want {
			t.Errorf("find %q = %v; want %q", test.args, got, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		word string
	}{
		{args: []string{".", "-perm", "+x"}, word: "invalid mode 'x'"},
		{args: []string{".", "-perm", "9"}, word: "invalid mode '9'"},
		{args: []string{".", "-samefile", "nope"}, word: "nope"},
		{args: []string{".", "-links", "two"}, word: "two"},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "find", test.args...)
		if err == nil || stdout != "" || !strings.Contains(stderr+err.Error(), test.word) {
			t.Errorf("find %q = %q, %q, %v; want a refusal naming %q", test.args, stdout, stderr, err, test.word)
		}
	}
}
