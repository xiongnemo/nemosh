package runtime_test

import "testing"

// `echo $(echo $((2*3)))` was a syntax error.
//
// The scan that finds the end of a `$( )` treated `$((` as one opening parenthesis,
// pushing a single level and stepping over a single character -- but `$((` opens two
// and closes two, so the count came up one short and the real closing parenthesis
// arrived unmatched. The logical-line scanner already had a branch for exactly this,
// with a comment explaining why; this scanner did not.
//
// Arithmetic inside a substitution is common enough that its absence is a wall:
// `count=$(echo $((a+b)))` and anything shaped like it.
func TestCommandSubstitution_holdsArithmeticInside(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "arithmetic inside", script: "echo $(echo $((2*3)))\n", want: "6\n"},
		{name: "assigned from it", script: "x=$(echo $((1+1)))\necho $x\n", want: "2\n"},
		{
			name: "with variables", script: "a=2\nb=3\necho $(echo $((a+b)))\n", want: "5\n",
		},
		{name: "twice in one substitution", script: "echo $(echo $((1+1)) $((2+2)))\n", want: "2 4\n"},
		{name: "nested parentheses inside the arithmetic", script: "echo $(echo $(( (1+2)*3 )))\n", want: "9\n"},
		{
			// The two scanners have to agree about a substitution inside a
			// substitution as well.
			name: "a substitution inside a substitution", script: "echo $(echo a $(echo b))\n", want: "a b\n",
		},
		{name: "a literal parenthesis inside", script: "echo $(echo \"(paren)\")\n", want: "(paren)\n"},
		{name: "an array assignment inside", script: "x=$(b=(3 4); echo ${b[1]})\necho $x\n", want: "4\n"},
		{name: "the backquote spelling still works", script: "echo `echo backquote`\n", want: "backquote\n"},
		{name: "arithmetic on its own is unaffected", script: "echo $((2*3))\n", want: "6\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
