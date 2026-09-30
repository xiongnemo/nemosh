package runtime_test

import "testing"

// A `++` or `--` that steps no variable is two signs, as both references read it: `0++1` is
// 0 + +1 and `(a)+++3` is (a) + + + 3. It was an increment of nothing, `unexpected "++"`, which
// ended the script. One beside a name still steps it. busybox's ash_test arith-postinc, and
// bash answers the same.
func TestRuntime_plusPlusWithoutAVariableIsTwoSigns(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"echo $((0++1)) $((0--1))\n", "1 1\n"},
		{"a=3\necho $((a+++3)) $a\necho $(((a)+++3))\n", "6 4\n7\n"},
		{"x=1; echo $((x++)) $x $((++x)) $x $((x--)) $((--x)) $x\n", "1 2 3 3 3 1 1\n"},
		{"a=(5); echo $((a[0]++)) ${a[0]}\n", "5 6\n"},
		{"echo $(( 2 - -1 )) $(( 2--1 )) $((1 + ++x)) $x\n", "3 3 2 1\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
