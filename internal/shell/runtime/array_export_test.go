package runtime_test

import "testing"

// An array is exported to no child, as in bash: it is marked, `declare -p` and `${a@a}` say
// so, and the environment does not have it. `a=(x); export a` gave a child a=x. And a name
// declared one kind of array is not made the other: `declare -A` over an indexed array
// emptied it, where bash refuses with 1 and keeps the elements. A scalar made an array is its
// element or key 0, as there. The answers are bash 5.3's; busybox has no arrays.
func TestRuntime_arraysAreNotExported(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"a=(mystr); export a; echo \"env: $(env | grep -c '^a=')\"; echo \"[${a@a}]\"\n", "env: 0\n[ax]\n"},
		{"a=x; export a; a=(1 2); echo \"env: $(env | grep -c '^a=')\"; declare -p a\n", "env: 0\ndeclare -ax a=([0]=\"1\" [1]=\"2\")\n"},
		{"declare -ax e=(1 2); echo \"env: $(env | grep -c '^e=')\"\n", "env: 0\n"},
		{"b=(1 2 3); declare -A b 2>&1; echo \"st=$?\"; declare -p b\n",
			"nemosh: line 1: declare: b: cannot convert indexed to associative array\nst=1\ndeclare -a b=([0]=\"1\" [1]=\"2\" [2]=\"3\")\n"},
		{"declare -A m=([x]=1); declare -a m 2>&1; echo \"st=$?\"; declare -p m\n",
			"nemosh: line 1: declare: m: cannot convert associative to indexed array\nst=1\ndeclare -A m=([x]=\"1\" )\n"},
		{"x=1; declare -A x; declare -p x\n", "declare -A x=([0]=\"1\" )\n"},
		{"x=1; declare -a x; declare -p x\n", "declare -a x=([0]=\"1\")\n"},
		{"l() { local -A b; echo \"local st=$?\"; }; b=(1); l\n", "local st=0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
