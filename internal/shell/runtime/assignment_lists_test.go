package runtime_test

import "testing"

// In an assignment nothing is split, so "$@" and an array's "${a[@]}" are one value
// there, not one word each. They were one word each, so `x="$@"` assigned the first
// parameter and ran the second as a command -- `set -- a b c; x="$@"` ran `b`.
//
// The two references join them differently, and each decides where it has the construct.
// busybox-w32 joins the parameters with IFS's first character, as it joins unquoted $@ in
// an assignment, so with IFS=: `x="$@"` is a:b:c there and a b c in bash. busybox has no
// arrays, and bash joins an array's elements with a space whatever IFS is.
func TestRuntime_assignmentJoinsAListIntoOneValue(t *testing.T) {
	tests := []struct {
		name, script, want string
	}{
		{"quoted $@", `set -- a b c; x="$@"; echo "[$x]"`, "[a b c]\n"},
		{"quoted $@ with IFS=:", `set -- a b c; IFS=:; x="$@"; echo "[$x]"`, "[a:b:c]\n"},
		{"quoted $@ with no parameters", `set --; x="$@"; echo "[$x]"`, "[]\n"},
		{"quoted $@ around other text", `set -- a b; x="<$@>"; echo "[$x]"`, "[<a b>]\n"},
		{"local", `f() { local v="$@"; echo "[$v]"; }; f p q`, "[p q]\n"},
		{"export", `set -- a b; export x="$@"; echo "[$x]"`, "[a b]\n"},
		{"quoted array", `a=(1 2); IFS=:; z="${a[@]}"; echo "[$z]"`, "[1 2]\n"},
		{"unquoted array", `a=(1 2); IFS=:; w=${a[@]}; echo "[$w]"`, "[1 2]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, status := runScriptCapturing(test.script)
			if stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
