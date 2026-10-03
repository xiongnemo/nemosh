package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// [[ ]] makes its redirections, as any command does: `[[ a == a ]] 2>x1.txt` creates x1.txt
// in busybox-w32 and bash, and a diagnostic of its own -- a test it cannot make, `a` being no
// number -- goes where 2> sends it. They were expanded and then dropped, so no file was made
// and `2>/dev/null` hid nothing.
func TestDoubleBracket_makesItsRedirections(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := "cd '" + dir + "'\n" +
		"[[ a == a ]] 2>x1.txt; echo st=$?\n" +
		"[[ a == b ]] > x2.txt; echo st=$?\n" +
		"[[ a -lt 1 ]] 2>x3.txt; echo st=$?\n"
	if stdout, status := runScriptCapturing(script); stdout != "st=0\nst=1\nst=2\n" || status != 0 {
		t.Fatalf("got %q/%d", stdout, status)
	}
	for _, name := range []string{"x1.txt", "x2.txt", "x3.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not created: %v", name, err)
		}
	}
	if diagnostic, err := os.ReadFile(filepath.Join(dir, "x3.txt")); err != nil || len(diagnostic) == 0 {
		t.Errorf("the malformed expression's diagnostic did not go to x3.txt: %q, %v", diagnostic, err)
	}
}
