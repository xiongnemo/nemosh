package applets_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ls takes the rest of busybox-w32's options: -p -x -g -n -i -s -Q -q -k -T, -c -u -X -v -L -H,
// --full-time and --group-directories-first, and of -C, -x and -l the last given wins. Columns
// are as wide as busybox's, one less than the terminal or 79, the widest name without its
// indicator and two, -w 0 no limit and -w alone no columns. `total` is the entries' blocks, and a
// directory's links on Windows are two and its subdirectories. Each answer is busybox-w32's,
// measured, with the names and times of the fixture below.
func TestLs_takesBusyboxsOptions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sub/deeper", "sub2"} {
		if err := os.MkdirAll(filepath.Join(dir, "d", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	for _, name := range []string{"a10", "a2", "a9", "x.tar.gz", "y.gz", "sp ace", "f.txt"} {
		path := filepath.Join(dir, "d", name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-p", "d"}, "a10\na2\na9\nf.txt\nsp ace\nsub/\nsub2/\nx.tar.gz\ny.gz\n"},
		{[]string{"-v", "d"}, "a2\na9\na10\nf.txt\nsp ace\nsub\nsub2\nx.tar.gz\ny.gz\n"},
		{[]string{"-X", "d"}, "a10\na2\na9\nsp ace\nsub\nsub2\ny.gz\nx.tar.gz\nf.txt\n"},
		{[]string{"-rX", "d"}, "f.txt\nx.tar.gz\ny.gz\nsub2\nsub\nsp ace\na9\na2\na10\n"},
		{[]string{"-Q", "d/sp ace"}, "\"d/sp ace\"\n"},
		{[]string{"--group-directories-first", "d"}, "sub\nsub2\na10\na2\na9\nf.txt\nsp ace\nx.tar.gz\ny.gz\n"},
		{[]string{"-x", "-w", "40", "d"}, "a10       a2        a9        f.txt\nsp ace    sub       sub2      x.tar.gz\ny.gz\n"},
		{[]string{"-C", "-w", "40", "d"}, "a10       f.txt     sub2\na2        sp ace    x.tar.gz\na9        sub       y.gz\n"},
		{[]string{"-l", "-C", "-w", "40", "d"}, "a10       f.txt     sub2\na2        sp ace    x.tar.gz\na9        sub       y.gz\n"},
		{[]string{"-1", "-x", "-w", "40", "d"}, "a10       a2        a9        f.txt\nsp ace    sub       sub2      x.tar.gz\ny.gz\n"},
		{[]string{"-C", "-F", "-w", "20", "d/sub", "d/f.txt"}, "d/f.txt\n\nd/sub:\ndeeper/\n"},
		{[]string{"-w", "40", "d/sub2", "d/sub"}, "d/sub:\ndeeper\n\nd/sub2:\n"},
		{[]string{"-w", "0", "-C", "d"}, "a10       a2        a9        f.txt     sp ace    sub       sub2      x.tar.gz  y.gz\n"},
		{[]string{"-k", "-T", "4", "d/f.txt"}, "d/f.txt\n"},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "ls", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("ls %q = %q (%v, %q), want %q", test.args, stdout, err, stderr, test.want)
		}
	}
	full := regexp.MustCompile(`^-\S{9}\s+1 \S+\s+0 2020-01-01 00:00:00 [-+]\d{4} d/f\.txt\n$`)
	if stdout, _, err := runSmall(t, dir, "", "ls", "-lg", "--full-time", "d/f.txt"); err != nil || !full.MatchString(stdout) {
		t.Errorf("ls -lg --full-time = %q, %v", stdout, err)
	}
	inode := regexp.MustCompile(`^ {0,18}\d+ d/f\.txt\n$`)
	if stdout, _, err := runSmall(t, dir, "", "ls", "-i", "d/f.txt"); err != nil || !inode.MatchString(stdout) || len(stdout) < 28 {
		t.Errorf("ls -i = %q, %v; want the inode in nineteen", stdout, err)
	}
	blocks := regexp.MustCompile(`^total \d+\n( {5}\d| {4}\d\d) a10\n`)
	if stdout, _, err := runSmall(t, dir, "", "ls", "-s", "d"); err != nil || !blocks.MatchString(stdout) {
		t.Errorf("ls -s = %q, %v; want a total and the blocks in six", stdout, err)
	}
	numeric := regexp.MustCompile(`^-\S{9}\s+1 \d+\s+\d+\s+0 Jan 01  2020 d/f\.txt\n$`)
	if stdout, _, err := runSmall(t, dir, "", "ls", "-n", "d/f.txt"); err != nil || !numeric.MatchString(stdout) {
		t.Errorf("ls -n = %q, %v", stdout, err)
	}
	if runtime.GOOS == "windows" {
		// busybox-w32's count_subdirs: two, and one for each directory in it.
		links := regexp.MustCompile(`^d\S{9}\s+3 .* d/sub\nd\S{9}\s+2 .* d/sub2\n$`)
		if stdout, _, err := runSmall(t, dir, "", "ls", "-ld", "d/sub", "d/sub2"); err != nil || !links.MatchString(stdout) {
			t.Errorf("ls -ld = %q, %v", stdout, err)
		}
		if stdout, _, err := runSmall(t, dir, "", "ls", "-s", "d/sub2", "d/sub"); err != nil || !strings.Contains(stdout, "d/sub:\ntotal 0\n     0 deeper\n") {
			t.Errorf("ls -s = %q, %v", stdout, err)
		}
	}
}
