package runtime_test

import "testing"

// Only a `${` opens a nested expansion inside `${...}`; a bare `{` is text, so the first `}`
// after it ends the expansion, in busybox-w32 and bash alike: `${x:-{b}}` with x set is a and
// a }, and `${X//a/{x,y,z}}` replaces with `{x,y,z` and adds the }. A bare `{` was counted as
// a nesting, the expansion ran on to the second `}`, and the } after it vanished.
func TestRuntime_onlyADollarBraceNestsInAnExpansion(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x=a; echo ${x:-{b}} ${u:-{b}}", "a} {b}\n"},
		{"x=a; echo \"${x:+{b}}\" ${x/a/{c}}", "{b} {c}\n"},
		{"x=a; echo ${x:-${y:-{c}}}", "a}\n"},
		{"IFS=x; X=a=\\\"\\$a\\\"; echo ${X//a/{x,y,z}}", "{ ,y,z=\"${ ,y,z\"}\n"},
		{"x=a; echo ${x:-\"{\"} ${x:-\\{}}", "a a}\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
