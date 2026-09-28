package runtime_test

import "testing"

// A quoted word is no array literal, as it is no assignment: `echo "a=(1 2)"` prints its
// operand in busybox-w32 and bash, and `"a=(1 2)"` alone is a command neither finds. The first
// was refused as an array literal after a command name, so the script never began; the second
// made an array.
func TestRuntime_quotedArrayLiteralIsAWord(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`echo "a=(1 2)"`, "a=(1 2)\n"},
		{`echo 'a=(1)' "b+=(2)"`, "a=(1) b+=(2)\n"},
		{`printf '%s\n' "x=(y)"`, "x=(y)\n"},
		{`"a=(1 2)" 2>/dev/null; echo $? ${#a[@]}`, "127 0\n"},
		{`'a=(1 2)' 2>/dev/null; echo $? ${#a[@]}`, "127 0\n"},
		{`a=(1 2); echo ${#a[@]}`, "2\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as both references answer", index, test.script, stdout, status, test.want)
		}
	}
}
