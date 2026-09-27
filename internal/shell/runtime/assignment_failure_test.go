package runtime_test

import "testing"

// An assignment statement that fails ends the script, status 1, as bash has it: an element
// past the front of an array, a list assigned to one element, and an integer's value that is
// no expression. With declare it is status 1 and the next line, as there, and so is a
// nameref's circle. The error was said and the script went on, with the list written over
// the whole array. busybox-w32 has no arrays and no integers to ask.
func TestRuntime_failedAssignmentEndsTheScript(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"a=(); a[-1]=1; echo after\n", "", 1},
		{"a=(1 2); a[-3]=x; echo after\n", "", 1},
		{"a=(1 2); v=x; a[-3]=$v; echo after\n", "", 1},
		{"a=(); a[-1]=1 b=2; echo after\n", "", 1},
		{"a=(1 2); a[0]=(3 4); echo after\n", "", 1},
		{"declare -i n; n='1+'; echo after\n", "", 1},
		{"a=(1); declare a[-5]=x; echo \"after $?\"\n", "after 1\n", 0},
		{"typeset -n r1=r2; typeset -n r2=r1; r1=z; echo \"after $?\"\n", "after 1\n", 0},
		{"a=(1 2); a[-2]=x; echo \"after $? ${a[@]}\"\n", "after 0 x 2\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as bash answers", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
