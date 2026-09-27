package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A subscript and an element are expanded as bash expands them. The value of a `[k]=value`
// element is an assignment's value, neither brace-expanded nor globbed: `a=([0]=*.py)` is the
// star, where it globbed and joined the matches, and `[k]=-{a,b}-` is those seven characters,
// where it was `-a- -b-`. A double-quoted subscript of an indexed array is arithmetic once its
// quotes are gone, so `${a["0"]}` is element 0; it was nothing. And unset expands an associative
// subscript as a word: `unset 'm[$k]'` removes the key k holds, where it looked for the key
// spelled `$k` and removed nothing. The answers are bash 5.3's; busybox has no arrays.
func TestRuntime_arraySubscriptsAndElementsExpandAsInBash(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"touch a.py b.py; x=({a,b}.py [3]=*.py [5]=-{a,b}-); declare -p x", "declare -a x=([0]=\"a.py\" [1]=\"b.py\" [3]=\"*.py\" [5]=\"-{a,b}-\")\n"},
		{"declare -A m=([k]=-{a,b}- [q]={1..3}); echo \"${m[k]} ${m[q]}\"", "-{a,b}- {1..3}\n"},
		{"a=([k1]=v1 [k2]=v2); echo \"${a[\"k1\"]} ${a[\"0\"]}\"", "v2 v2\n"},
		{"a=(x y z); i=1; echo \"${a[\"$i\"]} ${a[\"i+1\"]}\"; unset 'a[\"1\"]'; echo \"${!a[@]}\"", "y z\n0 2\n"},
		{"declare -A d=([k]=v [x]=y); key=k; unset 'd[$key]'; echo \"${!d[@]}\"", "x\n"},
		{"declare -A d=([k]=v [x]=y); key=k; unset 'd[\"$key\"]'; echo \"${!d[@]}\"", "x\n"},
		{"declare -A d=([\"a b\"]=v [x]=y); unset 'd[\"a b\"]'; echo \"${!d[@]}\"", "x\n"},
		{"declare -A d=(); key='1],a[1'; d[\"$key\"]=foo; unset -v 'd[\"$key\"]'; echo \"${#d[@]}\"", "0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			script := fmt.Sprintf("cd '%s'\n%s\n", filepath.ToSlash(t.TempDir()), test.script)
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
