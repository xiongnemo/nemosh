package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// exit, return, break and continue make their redirections as any command does, though
// nothing is written through them: `break > r1.txt` creates r1.txt in both references. None
// of them was made.
func TestRuntime_controlBuiltinsMakeTheirRedirections(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := "cd '" + dir + "'\n" +
		"for x in a; do break > r1.txt; done\n" +
		"for x in a; do continue 2> r2.txt; done\n" +
		"f() { return 3 > r3.txt; }; f; echo f=$?\n" +
		"(exit 4 > r4.txt); echo sub=$?\n"
	if stdout, status := runScriptCapturing(script); stdout != "f=3\nsub=4\n" || status != 0 {
		t.Fatalf("got %q/%d, want %q/0", stdout, status, "f=3\nsub=4\n")
	}
	for _, name := range []string{"r1.txt", "r2.txt", "r3.txt", "r4.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not created: %v", name, err)
		}
	}
}
