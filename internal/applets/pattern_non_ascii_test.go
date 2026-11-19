package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// A pattern's characters outside ASCII are matched as themselves. The translations of a basic
// regexp -- grep's and sed's -- of awk's escapes and of find's glob copied a byte of one as the
// character of that number, `中` as `ä¸­`, so `grep 中` matched nothing, `sed s/中/X/` changed
// nothing and `find -path '*文件*'` found nothing, where busybox-w32 finds and changes them all.
// grep -E and -F were right, as they hand Go the pattern as it is.
func TestPatterns_matchCharactersOutsideASCII(t *testing.T) {
	text := "中文测试\nplain line\ncafé au lait\n"
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{"grep", []string{"中"}, "中文测试\n"},
		{"grep", []string{"-w", "中文测试"}, "中文测试\n"},
		{"grep", []string{`caf\é`}, "café au lait\n"},
		{"sed", []string{"s/中/X/"}, "X文测试\nplain line\ncafé au lait\n"},
		{"sed", []string{"-n", "/测试/p"}, "中文测试\n"},
		{"sed", []string{"s/é/E/"}, "中文测试\nplain line\ncafE au lait\n"},
		{"awk", []string{`/caf\é/`}, "café au lait\n"},
	} {
		if stdout, stderr, status := runApplet(t, test.applet, test.args, text); stdout != test.want || status != 0 {
			t.Errorf("%s %q: %q, %q, status %d; want %q", test.applet, test.args, stdout, stderr, status, test.want)
		}
	}

	dir := filepath.ToSlash(t.TempDir())
	if err := os.WriteFile(filepath.FromSlash(dir+"/文件.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, status := runApplet(t, "find", []string{dir, "-path", "*文件*"}, ""); stdout != dir+"/文件.txt\n" || status != 0 {
		t.Errorf("find -path '*文件*': %q, %q, status %d; want the file", stdout, stderr, status)
	}
}
