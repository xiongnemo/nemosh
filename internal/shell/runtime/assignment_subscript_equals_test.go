package runtime_test

import "testing"

// An element assignment's subscript may hold an `=` of its own, as bash reads it: the
// assignment's is the first one past the `]`. busybox-w32 has no arrays. `a[x=1]=one` ran as
// a command of that name, and declare and local took `a[x` for the name.
func TestRuntime_elementAssignmentSubscriptHoldsEquals(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`a[x=1]=one; echo "x=$x ${a[1]}"`, "x=1 one\n"},
		{`declare a[x=1]=one; echo "x=$x ${a[1]}"`, "x=1 one\n"},
		{`declare -A A; A[k=v]=w; echo "${!A[@]} ${A[k=v]}"`, "k=v w\n"},
		{`f() { local a[y=1]=one; echo "y=$y ${a[1]}"; }; f`, "y=1 one\n"},
		{`a[x=1]+=one; a[x=1]+=two; echo "${a[1]}"`, "onetwo\n"},
		{`a[b[1]=2]=z; echo "${!a[@]} ${b[1]}"`, "2 2\n"},
		{`x=5; a[x==5]=t; echo "${!a[@]}"`, "1\n"},
		{`code='y=2'; a[$code]=two; echo "y=$y ${a[2]}"`, "y=2 two\n"},
		{`a[1]=b=c; x=a[1]; echo "${a[1]} $x"`, "b=c a[1]\n"},
		// In a function, local and declare make an element's array the call's own, as they do
		// a name: `local a[1]=l` was a bad variable name, and `declare a[2]=two` set a global.
		{`f() { local a[3]=4 a[5]=6; echo "status=$? ${!a[@]} ${a[@]}"; }; f`, "status=0 3 5 4 6\n"},
		{`a=(g); f() { local a[1]=l; echo "in ${a[@]}"; }; f; echo "out ${a[@]}"`, "in l\nout g\n"},
		{`f() { declare a[2]=two; echo "in ${a[2]}"; }; f; echo "out [${a[2]}]"`, "in two\nout []\n"},
		{`f() { local a[1]; echo "in ${#a[@]} ${a[1]-unset}"; }; f`, "in 0 unset\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
