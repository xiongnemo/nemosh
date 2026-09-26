package runtime_test

import "testing"

// An operator applied to one array element works on that element: `${a[1]:-d}` is a[1]
// when it is set and not empty. Every operator read the element as unset, since the
// operator's name lookup knew scalars only, so `:-` always gave the default, `#` and `/`
// always gave nothing, and `:=` refused outright. A `+` inside the subscript was also
// taken for the operator, so `${a[i+0]:-d}` split in the wrong place. The answers are
// bash 5.3's; busybox-w32 has no arrays.
func TestRuntime_operatorOnAnArrayElementWorksOnTheElement(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=(xa yb); echo "${a[1]:-d}"`, "yb\n"},
		{`a=(xa ""); echo "${a[1]:-d}" "[${a[1]-d}]"`, "d []\n"},
		{`a=(xa); echo "${a[5]:-d}" "${a[5]-e}"`, "d e\n"},
		{`a=(xa yb); echo "${a[1]:+set}" "[${a[4]:+set}]"`, "set []\n"},
		{`a=(xa); echo "${a[3]:=z}"; echo "${a[3]}" ${#a[@]}`, "z\nz 2\n"},
		{`declare -A m=([k]=v); echo "${m[k]:-d}" "${m[x]:-d}"`, "v d\n"},
		{`a=(xa yb); echo "${a[1]#y}" "${a[1]%b}"`, "b y\n"},
		{`a=(xa yb); echo "${a[1]/y/Y}"`, "Yb\n"},
		{`a=(xa yb); echo "${a[1]:1}"`, "b\n"},
		{`a=(xa yb); i=1; echo "${a[i]:-d}" "${a[$i]:-d}" "${a[i+0]:-d}"`, "yb yb yb\n"},
		{`a=(xa yb); echo "${a[1]^^}"`, "YB\n"},
		{`set -u; a=(xa); echo "${a[3]:-d}"`, "d\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
