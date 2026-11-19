package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// tree is tree(1)'s, from its source (tree.c, list.c, unix.c, color.c): the UTF-8 lines a UTF-8
// locale draws -- a bar and two no-break spaces, then a space -- names in byte order, dotfiles
// left out but with -a, and `N directories, M files` after a blank line. It ran Windows'
// tree.com, which answers `tree -L 2` with `Too many parameters`.
func TestTree_drawsAsTreeDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"d/a.txt": "hello", "d/sub/b.txt": "x", "d/sub/deeper/x": "", "d/z.txt": "", "d/.hidden": ""})
	bar := "│\xc2\xa0\xc2\xa0 " // a bar, two no-break spaces and a space, as tree's UTF-8 table has it
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"d"}, "d\n├── a.txt\n├── sub\n" + bar + "├── b.txt\n" + bar + "└── deeper\n" + bar + "    └── x\n└── z.txt\n\n2 directories, 4 files\n"},
		{[]string{"-a", "d"}, "d\n├── .hidden\n├── a.txt\n├── sub\n" + bar + "├── b.txt\n" + bar + "└── deeper\n" + bar + "    └── x\n└── z.txt\n\n2 directories, 5 files\n"},
		{[]string{"-d", "d"}, "d\n└── sub\n    └── deeper\n\n2 directories\n"},
		{[]string{"-L", "1", "d"}, "d\n├── a.txt\n├── sub\n└── z.txt\n\n1 directory, 2 files\n"},
		{[]string{"-f", "-L", "1", "d"}, "d\n├── d/a.txt\n├── d/sub\n└── d/z.txt\n\n1 directory, 2 files\n"},
		{[]string{"-i", "d"}, "d\na.txt\nsub\nb.txt\ndeeper\nx\nz.txt\n\n2 directories, 4 files\n"},
		{[]string{"--charset=ascii", "d"}, "d\n|-- a.txt\n|-- sub\n|   |-- b.txt\n|   `-- deeper\n|       `-- x\n`-- z.txt\n\n2 directories, 4 files\n"},
		{[]string{"-I", "sub|*.txt", "d"}, "d\n\n0 directories, 0 files\n"},
		{[]string{"-P", "*.txt", "d"}, "d\n├── a.txt\n├── sub\n" + bar + "├── b.txt\n" + bar + "└── deeper\n└── z.txt\n\n2 directories, 3 files\n"},
		{[]string{"--dirsfirst", "d"}, "d\n├── sub\n" + bar + "├── deeper\n" + bar + bar + "└── x\n" + bar + "└── b.txt\n├── a.txt\n└── z.txt\n\n2 directories, 4 files\n"},
		{[]string{"-F", "-d", "d"}, "d/\n└── sub/\n    └── deeper/\n\n2 directories\n"},
		{[]string{"--noreport", "-L", "1", "d"}, "d\n├── a.txt\n├── sub\n└── z.txt\n"},
		{[]string{"-h", "-P", "a.txt", "-L", "1", "--noreport", "d"}, "[" + dirSize(t, dir, "-h") + "]  d\n├── [   5]  a.txt\n└── [" + dirSize(t, dir+"/d", "-h") + "]  sub\n"},
		{[]string{"-s", "-P", "a.txt", "-L", "1", "--noreport", "d"}, "[" + dirSize(t, dir, "-s") + "]  d\n├── [          5]  a.txt\n└── [" + dirSize(t, dir+"/d", "-s") + "]  sub\n"},
	} {
		if stdout, stderr, err := runSmall(t, dir, "", "tree", test.args...); err != nil || stdout != test.want {
			t.Errorf("tree %q = %q, %v (%s);\nwant %q", test.args, stdout, err, stderr, test.want)
		}
	}
	stdout, _, err := runSmall(t, dir, "", "tree", "nope")
	if status, _ := applets.StatusCode(err); stdout != "nope [error opening dir]\n\n0 directories, 0 files\n" || status != 2 {
		t.Errorf("tree nope = %q, %v; want the error in tree's words and status 2", stdout, err)
	}
	for _, flag := range []string{"/F", "/a"} {
		if _, _, err := runSmall(t, dir, "", "tree", flag); err == nil || !strings.Contains(err.Error(), "tree.com") {
			t.Errorf("tree %s = %v; want a pointer to tree.com", flag, err)
		}
	}
}

// dirSize is the size column tree's -s or -h writes for the directory under parent named d or
// sub, which on Windows is what stat says a directory's size is.
func dirSize(t *testing.T, parent, flag string) string {
	t.Helper()
	name := "d"
	if strings.HasSuffix(filepath.ToSlash(parent), "/d") {
		name = "sub"
	}
	info, err := os.Stat(filepath.Join(parent, name))
	if err != nil {
		t.Fatal(err)
	}
	size := info.Size()
	if flag == "-h" && size < 1024 {
		return strings.Repeat(" ", 4-len(itoa(size))) + itoa(size)
	}
	if flag == "-s" {
		return strings.Repeat(" ", 11-len(itoa(size))) + itoa(size)
	}
	t.Fatalf("a directory of %d bytes, which this helper does not render for %s", size, flag)
	return ""
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}
