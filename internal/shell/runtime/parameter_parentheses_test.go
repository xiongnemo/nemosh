package runtime_test

import "testing"

// A parenthesis inside ${...} belongs to the expansion: `${x:(-1)}` is bash's and busybox's
// way to write a negative offset without the space, and `${v:-(none)}` a default that
// happens to hold parentheses. Unquoted, each was refused as "unsupported syntax: grouping"
// and stopped the whole script; only the quoted spelling worked. busybox-w32 and bash 5.3
// agree on each; the array slice is bash's.
func TestRuntime_parenthesisInsideAnExpansionIsTheExpansions(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x=hello; echo ${x:(-1)}`, "o\n"},
		{`x=hello; echo ${x:(-2):1}`, "l\n"},
		{`x=hello; echo ${x:(1+1)}`, "llo\n"},
		{`x=hello; echo "${x:(-3)}"`, "llo\n"},
		{`echo ${undefined:-(default)}`, "(default)\n"},
		{`a=(p q r); echo ${a[@]:(-2)}`, "q r\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
