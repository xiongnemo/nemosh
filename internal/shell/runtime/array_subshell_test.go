package runtime_test

import (
	"strings"
	"testing"
)

// Arrays across a subshell boundary.
//
// The array store was left out of the snapshot constructor entirely, so every
// snapshot had a nil one and any array assignment inside a subshell, a command
// substitution or a pipeline stage died on a nil map write -- a Go stack trace
// where a shell should have printed a number. `(a=(1 2))` was enough to do it.
//
// The scoping is measured against bash: a subshell sees the parent's arrays and a
// write inside one does not escape.
func TestArrays_crossSubshellBoundariesWithoutCrashing(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "assigned inside a subshell",
			script: "(a=(1 2); echo ${a[0]})\n", want: "1\n",
		},
		{
			name:   "assigned inside a pipeline stage",
			script: "echo z | { c=(5 6); echo ${c[1]}; }\n", want: "6\n",
		},
		{
			// The parenthesis of an array assignment is part of a word, and the
			// command substitution scanner was the fifth layer to need telling:
			// it popped its own nesting at `4)` and then called the real `)`
			// missing.
			name:   "assigned inside a command substitution",
			script: "x=$(b=(3 4); echo ${b[1]})\necho \"[$x]\"\n", want: "[4]\n",
		},
		{
			name:   "inherited by a subshell",
			script: "a=(one two); (echo ${a[1]})\n", want: "two\n",
		},
		{
			name:   "inherited by a command substitution",
			script: "a=(one two)\necho \"[$(echo ${a[0]})]\"\n", want: "[one]\n",
		},
		{
			name:   "a write inside a subshell does not escape",
			script: "a=(1 2); (a[0]=9); echo ${a[0]}\n", want: "1\n",
		},
		{
			// Appending in a subshell must not write through a shared backing
			// slice into the parent, which is why the elements are copied and not
			// just the map.
			name:   "appending inside a subshell does not escape",
			script: "a=(1); (a+=(2); echo \"inside=${#a[@]}\"); echo \"outside=${#a[@]}\"\n",
			want:   "inside=2\noutside=1\n",
		},
		{
			name:   "unset inside a subshell does not escape",
			script: "a=(1 2); (a[0]=x); echo \"${#a[@]}\"\n", want: "2\n",
		},
		{
			name:   "a background job with an array",
			script: "a=(1 2); (echo ${a[0]}) & wait\n", want: "1\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if strings.Contains(stderr, "panic") {
				t.Fatalf("the shell panicked: %s", stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
