package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Text that eval or . reads and that does not parse is a syntax error, which ends a script, `||
// echo handled` or not, as busybox ends it and POSIX 2.8.1 has it. It went on with 2, as bash
// goes on. A subshell or a command substitution ends only itself, and under command only the
// eval ends.
func TestRuntime_aSyntaxErrorInEvalEndsTheScript(t *testing.T) {
	source := filepath.Join(t.TempDir(), "bad.sh")
	if err := os.WriteFile(source, []byte("if\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{
		"eval 'if'\necho reached\n",
		"eval 'if' || echo handled\necho reached\n",
		"f() { eval 'if'; echo in; }\nf\necho reached\n",
		". '" + filepath.ToSlash(source) + "'\necho reached\n",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
				t.Errorf("got %q/%d, want the script ended with 2, as busybox ends it", stdout, status)
			}
		})
	}
	for _, test := range []struct{ script, want string }{
		{"command eval 'if'\necho \"st=$?\"\n", "st=2\n"},
		{"x=$(eval 'if')\necho \"st=$?\"\n", "st=2\n"},
		{"(eval 'if')\necho \"st=$?\"\n", "st=2\n"},
		{"eval ''\necho \"st=$?\"\n", "st=0\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
}
