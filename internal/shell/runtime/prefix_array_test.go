package runtime_test

import "testing"

// An array assignment in front of a command is no array assignment, as bash 5.3 reads it: a
// list is the command's temporary string, `(1 2)`, and gone after it, and an element is not a
// valid identifier, said and dropped, the command run all the same. Both were made as arrays
// for good. busybox-w32 has no arrays.
func TestRuntime_prefixArrayAssignmentIsTemporary(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"f() { echo \"in ${a[@]}\"; }; a=(1 2) f; echo \"after [${a[@]}]\"\n", "in (1 2)\nafter []\n"},
		{"a=(1 2) true; echo \"[${a[@]}]\"\n", "[]\n"},
		{"a=(9); f() { echo \"in ${a[@]}\"; }; a[0]=x f; echo \"after ${a[@]}\"\n", "in 9\nafter 9\n"},
		{"a=(1 2); echo \"${a[@]}\"\n", "1 2\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
