package runtime_test

import "testing"

// An assignment statement that fails abandons the command the shell was running, status 1,
// as bash has it -- the function it called and the loop it was in with it -- and the script
// goes on with the next command, unless `set -e` ends it there: an element before the front of
// an array, a list assigned to one element, an integer's value that is no expression. With
// declare it is status 1 and the next command, as there, and so is a nameref's circle. The
// error was said and the command went on, with the list written over the whole array.
// busybox-w32 has no arrays and no integers to ask.
func TestRuntime_failedAssignmentAbandonsTheCommand(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"a=()\na[-1]=1\necho \"after $?\"\n", "after 1\n", 0},
		{"a=(1 2)\na[-3]=x\necho \"after $?\"\n", "after 1\n", 0},
		{"a=(1 2)\nv=x\na[-3]=$v\necho \"after $?\"\n", "after 1\n", 0},
		{"a=()\na[-1]=1 b=2\necho \"after $? [$b]\"\n", "after 1 []\n", 0},
		{"a=(1 2)\na[0]=(3 4)\necho \"after $? ${a[@]}\"\n", "after 1 1 2\n", 0},
		{"declare -i n\nn='1+'\necho \"after $?\"\n", "after 1\n", 0},
		{"f() {\n  a=(); a[-1]=1\n  echo in-f\n}\nf\necho \"after $?\"\n", "after 1\n", 0},
		{"for i in 1 2; do\n  a=(); a[-1]=$i\n  echo \"in $i\"\ndone\necho \"after $?\"\n", "after 1\n", 0},
		{"set -e\na=()\na[-1]=1\necho after\n", "", 1},
		{"a=(1); declare a[-5]=x; echo \"after $?\"\n", "after 1\n", 0},
		{"typeset -n r1=r2; typeset -n r2=r1; r1=z; echo \"after $?\"\n", "after 1\n", 0},
		{"a=(1 2); a[-2]=x; echo \"after $? ${a[@]}\"\n", "after 0 x 2\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as bash answers", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
