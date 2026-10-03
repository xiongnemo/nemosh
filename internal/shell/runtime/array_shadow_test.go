package runtime_test

import "testing"

// A temporary assignment to an array's name is a string for the command, in its environment
// too, and the array is as it was after it, as bash has `A=x cmd`; what the command did to the
// other arrays stays. The string went into the array's first element, for good, and the
// command's environment had no A: pip's completion never saw its words. Each line is bash 5.3's.
func TestArrayShadow_aTemporaryAssignmentToAnArraysNameIsAStringForTheCommand(t *testing.T) {
	script := `A=(1 2); A=x true; echo "after builtin: ${A[*]}"
A=x env | grep '^A='; echo "after env: ${A[*]}"
f() { echo "in f: $A ${A[1]}"; B[0]=changed; }
B=(b); A=x f; echo "after f: ${A[*]} B=${B[*]}"
declare -A M=([k]=v); M=s env | grep '^M='; echo "M: ${M[k]}"
`
	want := "after builtin: 1 2\nA=x\nafter env: 1 2\nin f: x \nafter f: 1 2 B=changed\nM=s\nM: v\n"
	if status, stdout, stderr := runSetScript(t, script); status != 0 || stdout != want || stderr != "" {
		t.Errorf("got %d/%q/%q, want 0 and %q", status, stdout, stderr, want)
	}
}
