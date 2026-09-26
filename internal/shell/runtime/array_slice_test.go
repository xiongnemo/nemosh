package runtime_test

import "testing"

// A slice of an indexed array is taken by subscript: the offset is the first index to
// include, so a sparse array is sliced where its elements are, and a negative offset counts
// back from one past the highest index. An offset that reaches back past the start gives
// nothing, for the positional parameters too. It counted positions instead, so a slice of a
// sparse array missed its elements, and an offset before the start was moved up to it. The
// answers are bash 5.3's; busybox-w32 has no arrays, and slices "$@" as one string.
func TestRuntime_arraySliceCountsBySubscript(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"a[33]=1; a[66]=2; a[99]=2\nprintf '[%s]' \"${a[@]:15:2}\"", "[1][2]"},
		{"(( a[33]=1 ))\n(( a[66]=2 ))\n(( a[99]=2 ))\nprintf '[%s]' \"${a[@]:15:2}\"", "[1][2]"},
		{"a=([2]=x [5]=y [9]=z); echo \"${a[@]: -4}|${a[@]: -8:2}|${a[@]:3}|${a[@]:10}|${a[@]:9}\"", "z|x y|y z||z\n"},
		{"a=(1 2 3 4)\necho \"(${a[*]: -4})\" \"(${a[*]: -5})\"", "(1 2 3 4) ()\n"},
		{"a=(x y z); unset 'a[1]'; echo \"${a[@]:1}|${a[@]:1:1}|${a[*]: -1}\"", "z|z|z\n"},
		{"set -- 1 2 3 4; echo \"(${@: -2})(${@: -6})(${*:2:2})\"", "(3 4)()(2 3)\n"},
		{"declare -A m=([k]=v); echo \"${m[@]:0:1}\"", "v\n"},
		// An empty slice of a `*` form is one empty word; the caller had no value to take.
		{"a=(1 2); echo \"[${a[*]:5}]\" \"[${a[*]:1:0}]\"; set -- a b; echo \"[${*:5}]\"", "[] []\n[]\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// A negative length is an error for a list, where for a string it counts from the end. It
// counted from the end for a list too, so the command ran on a slice bash would not have
// made. bash runs nothing and reports `substring expression < 0`; here it is an expansion
// error like any other, and ends the script.
func TestRuntime_listSliceRefusesANegativeLength(t *testing.T) {
	for _, script := range []string{
		"a=(1 2 3 4 5)\nprintf '[%s]' \"${a[@]: 1: -3}\"\necho after",
		"set -- 1 2 3 4 5\nprintf '[%s]' \"${@: 1: -3}\"\necho after",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status == 0 {
				t.Errorf("got %q/%d, want nothing run and a failing status", stdout, status)
			}
		})
	}
}
