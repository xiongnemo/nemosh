package runtime_test

import "testing"

// An indexed array holds the elements it has and nothing else, to the largest index an int64
// holds, as bash's does: `a[0x7FFFFFFFFFFFFFFF]=a` is one element more, not a slice grown to
// it -- that assignment hung, and `a[1000000000000]=x` could not have been made at all.
// Appending goes after the highest index set, and a negative subscript counts back from it.
// The answers are bash 5.3's; busybox has no arrays.
func TestRuntime_indexedArraysAreSparse(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{
			"sp[1]=x; sp[5]=y; sp[9]=z; echo ${#sp[@]}\nsp[0x7FFFFFFFFFFFFFFF]=a; echo ${#sp[@]} ${!sp[@]}\necho \"${sp[9223372036854775807]}\"\n",
			"3\n4 1 5 9 9223372036854775807\na\n",
		},
		{"d[1000000000000]=big; echo ${#d[@]} ${!d[@]}\n", "1 1000000000000\n"},
		{"a=(1 2); unset 'a[1]'; a+=(x); echo ${!a[@]}\nb[3]=z; b+=(w); echo ${!b[@]}\ne=(); e+=(f); echo ${!e[@]}\n", "0 1\n3 4\n0\n"},
		{"c=(p q r); echo ${c[-1]}; unset 'c[2]'; echo ${c[-1]}\n", "r\nq\n"},
		{"sp[1]=x; sp[9]=z; declare -p sp\n", "declare -a sp=([1]=\"x\" [9]=\"z\")\n"},
		{"f() { local -a l=(1 2); l[7]=s; echo \"${!l[@]}\"; }; l=(o); f; echo \"${!l[@]} ${l[0]}\"\n", "0 1 7\n0 o\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
