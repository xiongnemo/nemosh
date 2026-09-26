package runtime_test

import "testing"

// `test -v name` and `[ -v name ]` ask whether a shell variable is set, as `[[ -v name ]]`
// does: empty counts as set, a subscript names one element -- an index is arithmetic, a key
// expands -- and a bare array name is its element zero. test had no -v, so every one of these
// was status 2. The answers are bash 5.3's; busybox-w32's test has no -v.
func TestRuntime_testVAsksTheShell(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`test -v str; echo $?; str=; test -v str; echo $?; [ -v str ]; echo $?`, "1\n0\n0\n"},
		{`array=(1 2 3 ''); test -v 'array[1]'; echo $?; test -v 'array[3]'; echo $?; test -v 'array[4]'; echo $?`, "0\n0\n1\n"},
		{`array=(1 2 3 ''); test -v 'array[1+1]'; echo $?; test -v 'array[4+1]'; echo $?`, "0\n1\n"},
		{`typeset -a a; test -v a; echo $?; test -v 'a[0]'; echo $?; a[0]=1; test -v a; echo $?; test -v 'a[1]'; echo $?; test -v 'a[x]'; echo $?`, "1\n1\n0\n1\n0\n"},
		{`typeset -A A; test -v A; echo $?; A['0']=x; test -v A; echo $?; test -v 'A[0]'; echo $?; test -v 'A[x]'; echo $?`, "1\n0\n0\n1\n"},
		{`typeset -A m=([empty]='' [k]=v); key=empty; test -v 'm[$key]'; echo $?; key=no; test -v 'm[$key]'; echo $?`, "0\n1\n"},
		{`x=1; test ! -v x; echo $?; test -v x -a -v nosuch; echo $?`, "1\n1\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
