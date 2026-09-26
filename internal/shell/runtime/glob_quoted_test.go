package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A quoted or escaped pattern character in a word is matched as itself when an unquoted one
// beside it makes the word a pattern. The pattern was the field's text, where the quoting had
// gone, so `\[???\]` was a bracket expression of question marks and matched nothing, and
// `*.[C\-D]` a range. busybox-w32 and bash agree on every answer here.
func TestRuntime_globKeepsQuotedCharactersLiteral(t *testing.T) {
	directory := filepath.ToSlash(t.TempDir())
	script := fmt.Sprintf(`cd '%s'; touch "[abc]" foo.- c.C "a b.txt"; mkdir "x-y"; touch "x-y/q.txt"
echo \[???\]
echo *.[C-D] *.[C\-D]
echo "["a* "*".C \*.C "a "*
x="*"; echo $x.C "$x".C
echo "x-y"/*.txt`, directory)
	want := "[abc]\nc.C c.C foo.-\n[abc] *.C *.C a b.txt\nc.C *.C\nx-y/q.txt\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, want)
	}
}
