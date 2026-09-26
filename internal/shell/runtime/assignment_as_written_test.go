package runtime_test

import "testing"

// Only a word written as an assignment is one. A word that expands to `name=value`, a quoted
// one and one with its `=` escaped are command names: each runs, or is not found, in
// busybox-w32 and bash, where every one of them was taken for an assignment and quietly made.
// The assignments counted from the words as written, not from their text once expanded.
func TestRuntime_onlyAnAssignmentWordAssigns(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`v='a=b'; $v 2>/dev/null; echo "st=$? [$a]"`, "st=127 []\n"},
		{`"c=d" 2>/dev/null; echo "st=$? [$c]"`, "st=127 []\n"},
		{`e\=f 2>/dev/null; echo "st=$? [$e]"`, "st=127 []\n"},
		{`{v,x}=X 2>/dev/null; echo "st=$? [$v$x]"`, "st=127 []\n"},
		// Unchanged: assignments as written, alone and in front of a command, and bash's
		// element assignments with a subscript that is quoted or expanded, appending too.
		{`a=1 b=2; echo "$a$b"; x=1 env | grep '^x='`, "12\nx=1\n"},
		{`declare -A A; A['x']='foo'; A['x']+='bar'; k=x; A[$k]+=z; echo "${A[x]}"`, "foobarz\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
