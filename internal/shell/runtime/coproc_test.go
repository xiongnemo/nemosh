package runtime_test

import (
	"strings"
	"testing"
)

// coproc is refused by name, with status 126 as the other refused builtins are, and only
// when it is reached. `coproc cat` was a command not found, and `coproc NAME { cat; }` a
// syntax error that stopped the whole script before its first line.
func TestCoproc_isRefusedByNameWhenReached(t *testing.T) {
	for _, script := range []string{
		"coproc cat\necho \"st=$?\"\n",
		"coproc NAME { cat; }\necho \"st=$?\"\n",
		"coproc W {\n  cat\n}\necho \"st=$?\"\n",
		"if true; then coproc { cat; }; fi\necho \"st=$?\"\n",
	} {
		status, stdout, stderr := runSetScript(t, script)
		if status != 0 || stdout != "st=126\n" || !strings.Contains(stderr, "coproc: not implemented") {
			t.Errorf("%q = %d %q %q, want the refusal and the script going on", script, status, stdout, stderr)
		}
	}
	if stdout, _ := runScriptCapturing("if false; then coproc { cat; }; fi\necho reached\n"); stdout != "reached\n" {
		t.Fatalf("an unreached coproc stopped the script: %q", stdout)
	}
}
