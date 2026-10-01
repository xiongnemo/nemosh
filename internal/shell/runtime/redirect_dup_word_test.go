package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A duplication's word is expanded as a file redirection's is, as busybox expands both: no
// field splitting and no pathname expansion, and "$@" joined. When `>&` has no number in
// front and the word is no descriptor, both streams go to a file of that name. The word was
// split and globbed: `>& 1[0]` duplicated descriptor 10, the one a file named 10 matched,
// and `>& $x` with a blank in x was refused as ambiguous. An empty word is no descriptor,
// as busybox says. Each answer was measured against busybox-w32 in an empty directory.
func TestRuntime_duplicationWordIsOneWord(t *testing.T) {
	for _, test := range []struct {
		script, want string
	}{
		{": > 10; echo one >& 1[0]; cat '1[0]'", "one\n"},
		{"x='sp ace'; echo two >& $x; cat 'sp ace'", "two\n"},
		{"set -- a b; echo three >& \"$@\"; cat 'a b'", "three\n"},
		{"x=2; exec 2>&1; echo four >& $x", "four\n"},
		{"echo five >& $empty; echo \"after $?\"", "after 1\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			script := fmt.Sprintf("cd '%s'\n%s\n", filepath.ToSlash(t.TempDir()), test.script)
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, test.want)
			}
		})
	}
}
