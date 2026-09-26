package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// An assignment word is neither brace-expanded nor globbed. `x={X,Y}` was two assignments,
// x=X and then x=Y, and `foo=*` in a directory holding files named foo=a and foo=b globbed the
// whole word and assigned foo=b; export and typeset did the same. busybox-w32 and bash both
// keep the word as written. A declaration utility's operand is still brace-expanded, which
// is bash's: `export y={X,Y}` is y=X and then y=Y there, and busybox has no brace expansion.
func TestRuntime_assignmentWordIsNotBracedOrGlobbed(t *testing.T) {
	directory := filepath.ToSlash(t.TempDir())
	tests := []struct {
		script, want string
	}{
		{fmt.Sprintf(`cd '%s'; touch foo=a foo=b; foo=*; echo "[$foo]"; unset foo; export foo=*; echo "[$foo]"; unset foo; typeset foo=*; echo "[$foo]"`, directory), "[*]\n[*]\n[*]\n"},
		{`x={X,Y}; echo "[$x]"; w=a{1..2}; echo "[$w]"; v=b{1..2} env | grep '^v='`, "[{X,Y}]\n[a{1..2}]\nv=b{1..2}\n"},
		{`export y={X,Y}; echo "[$y]"`, "[Y]\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as the references answer", stdout, status, test.want)
			}
		})
	}
}
