package runtime_test

import "testing"

// An array read as a scalar is its element 0 -- key 0, for an associative one -- and is unset
// when it has none, as bash reads it: `$m`, `${m}`, `${#m}`, `${m-x}` and every operator. `$m`
// of an associative array was empty whatever it held, so `m=x; m+=y` left y, the append
// finding nothing to append to; and `$a` of an indexed one was a copy of element 0 that
// `unset 'a[0]'` left behind. The answers are bash 5.3's; busybox has no arrays.
func TestRuntime_arrayScalarIsElementZero(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{"declare -A m=([0]=zero [k]=v)\necho \"[$m] [${m}] [${m:-d}] [${#m}] [${m-unset}]\"\n", "[zero] [zero] [zero] [4] [zero]\n", 0},
		{"declare -A e=([k]=v)\necho \"[${e-unset}] [${#e}]\"\n", "[unset] [0]\n", 0},
		{"declare -A m; m=x; echo \"[$m] [${m[0]}]\"; m+=y; echo \"[$m] [${m[0]}]\"\n", "[x] [x]\n[xy] [xy]\n", 0},
		{"declare -A m=([0]=z); echo \"[${m:1}] [${m/z/y}] [${m^^}] [${m@Q}]\"\n", "[] [y] [Z] ['z']\n", 0},
		{"declare -A m=([0]=z); x=$m; n=m; echo \"$x ${!n}\"\n", "z z\n", 0},
		{"declare -A m=([0]=z); f() { local -n r=m; echo \"[$r]\"; }; f\n", "[z]\n", 0},
		{"a=([1]=x); echo \"[${a-unset}] [${#a}] [$a] [${a:-d}]\"\n", "[unset] [0] [] [d]\n", 0},
		{"a=(x y); unset 'a[0]'; echo \"[${a-unset}] [$a]\"\n", "[unset] []\n", 0},
		{"a=([1]=x); set -u; echo \"[$a]\"; echo after\n", "", 2},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as bash answers", stdout, status, test.want, test.status)
			}
		})
	}
}
