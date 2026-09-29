package runtime_test

import "testing"

// && and || in arithmetic are 0 or 1, and the side they stop before is not evaluated, as C has
// it. `2 || 3` was 2, where busybox-w32 and bash answer 1. `0 && (x = 44)` assigned x, and so
// does busybox's, but C and bash leave x alone, and the untaken arm of ?: already did here.
func TestArithmetic_logicalOperatorsShortCircuit(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`echo $(( 2 || 3 )) $(( 2 || 0 )) $(( 0 || 3 )) $(( 2 && 3 )) $(( 0 && 3 )) $(( 0 || 0 ))`, "1 1 1 1 0 0\n"},
		{`x=11; (( 1 || (x = 22) )); echo $x; (( 0 || (x = 33) )); echo $x`, "11\n33\n"},
		{`x=33; (( 0 && (x = 44) )); echo $x; (( 1 && (x = 55) )); echo $x`, "33\n55\n"},
		{`y=1; z=$(( 0 && y++ )); echo "y=$y z=$z"; z=$(( 1 || y++ )); echo "y=$y z=$z"`, "y=1 z=0\ny=1 z=1\n"},
		{`echo $(( 1 || 1/0 )) $(( 0 && 1/0 ))`, "1 0\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
