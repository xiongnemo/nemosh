package runtime_test

import "testing"

// A negative subscript counts back from one past the highest index that is set, as bash
// has it: after `unset 'a[2]'` on (x y z), a[-1] is y. It counted from one past the highest
// index ever set, since unsetting an element leaves its slot in place, so a[-1] read and
// wrote the element that was gone, and `unset 'a[-1]'` -- the way to drop the last element
// -- did nothing at all. One that reaches past the start is refused, with status 1. The
// answers are bash 5.3's; busybox-w32 has no arrays.
func TestRuntime_negativeSubscriptCountsFromTheLastSetIndex(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=(0 1 2 3); unset 'a[-1]'; echo len=${#a[@]}; unset 'a[-1]'; echo len=${#a[@]}`, "len=3\nlen=2\n"},
		{`a=(x); a[9]=y; unset -v 'a[-1]'; echo "len ${#a[@]}; last ${a[@]: -1}"`, "len 1; last x\n"},
		{`a=(x y z); unset 'a[2]'; echo "${a[-1]}"`, "y\n"},
		{`a=(x y z); unset 'a[2]'; a[-1]=Y; echo "${a[@]}"`, "x Y\n"},
		{`a=(x y); unset 'a[-5]' 2>/dev/null; echo "status=$?"`, "status=1\n"},
		{`a=(x y z); unset 'a[1]'; echo "${a[-2]}" "${#a[@]}"`, " 2\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
