package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A redirection with nothing after it on its line is a syntax error, "unexpected newline" in
// busybox's words, as both references refuse it. The line went on, and the next line's first
// word was taken for the file: `echo hi >` then `out.txt` wrote hi to out.txt.
func TestRuntime_aRedirectionWithNoTargetOnItsLineIsASyntaxError(t *testing.T) {
	dir := t.TempDir()
	for _, operator := range []string{">", ">>", "2>", "<", "<<", "| cat >"} {
		t.Run(operator, func(t *testing.T) {
			script := "cd '" + filepath.ToSlash(dir) + "'\necho hi " + operator + "\nout.txt\necho next\n"
			if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
				t.Errorf("got %q/%d, want the script refused with 2, as both references refuse it", stdout, status)
			}
			if _, err := os.Stat(filepath.Join(dir, "out.txt")); err == nil {
				t.Errorf("out.txt was written: the next line was taken for the target")
			}
		})
	}
}
