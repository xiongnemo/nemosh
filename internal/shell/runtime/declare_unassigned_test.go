package runtime_test

import (
	"strings"
	"testing"
)

// `declare -a a` and `local -A m` with no value leave an array declared and unset, as bash
// leaves it: declare -p writes it with no value, and `a=()` is the one written `=()`.
// Anything stored in it makes it set, an element later unset or not. It was `=()` for both.
// busybox has no arrays; bash decides, as the user chose.
func TestRuntime_declaredArrayHasNoValueUntilAssigned(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"declare -a a; declare -A m; declare -ai n; declare -p a m n", "declare -a a\ndeclare -A m\ndeclare -ai n\n"},
		{"f() { local -a la; local -A lm; declare -p la lm; }; f", "declare -a la\ndeclare -A lm\n"},
		{"declare -a e=(); t=(); declare -p e t", "declare -a e=()\ndeclare -a t=()\n"},
		{"declare -a a; a+=(x); declare -A m; m[k]=v; declare -p a m", "declare -a a=([0]=\"x\")\ndeclare -A m=([k]=\"v\" )\n"},
		{"declare -a u; u[3]=y; unset 'u[3]'; declare -p u", "declare -a u=()\n"},
		{"declare -a s; declare -a s; [[ -v s ]]; echo \"$? ${#s[@]}\"; declare -p s", "1 0\ndeclare -a s\n"},
		{"x=5; declare -a x; declare -p x", "declare -a x=([0]=\"5\")\n"},
		{"declare -a s; declare -p | grep ' s'", "declare -a s\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// Under `set -u` an array element that is not there is unset, and so is the count of a name
// never set or only declared, as bash has them; each was the empty string or 0, and the
// script went on. The words and the status are busybox's for any unset parameter. `${a[@]}`
// and an operator's word stay quiet, as in bash.
func TestRuntime_nounsetReachesArrayElements(t *testing.T) {
	for _, test := range []struct {
		script, unset string
	}{
		{"a=(1); echo \"${a[5]}\"", "a[5]"},
		{"a=(1); i=5; echo \"${a[$i]}\"", "a[$i]"},
		{"declare -A m=([k]=v); echo \"${m[j]}\"", "m[j]"},
		{"echo \"${nope[0]}\"", "nope[0]"},
		{"echo \"${#nope[@]}\"", "nope"},
		{"declare -a z; echo \"${#z[*]}\"", "z"},
		{"declare -A m; echo \"${#m[@]}\"", "m"},
		{"echo \"${#nope[0]}\"", "nope[0]"},
	} {
		t.Run(test.script, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, "set -u\n"+test.script+"\necho after\n")
			if status != 2 || stdout != "" || !strings.Contains(stderr, test.unset+": parameter not set") {
				t.Errorf("got %d, %q, %q; want 2, nothing and %q not set", status, stdout, stderr, test.unset)
			}
		})
	}
	quiet := "set -u\na=(1); declare -a z; echo \"[${a[5]:-d}] [${z[@]}] [${!z[@]}] [${#a[5]}] [${nope[@]}]\"\n"
	if status, stdout, stderr := runSetScript(t, quiet); status != 0 || stdout != "[d] [] [] [0] []\n" || stderr != "" {
		t.Errorf("got %d, %q, %q; want 0 and every form quiet", status, stdout, stderr)
	}
}
