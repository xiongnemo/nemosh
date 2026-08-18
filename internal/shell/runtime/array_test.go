package runtime_test

import (
	"testing"
)

// Indexed arrays, every expectation measured from bash.
//
// The one that carries the feature is `"${a[@]}"` against `"${a[*]}"`: the first
// is one word per element, so an element containing a blank survives, and the
// second is a single joined word. Without the first there would be no reason to
// have arrays -- a string would do.
func TestRuntime_arrays(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "an element by index", script: "a=(one two three)\necho ${a[0]}\n", want: "one\n"},
		{name: "the last element", script: "a=(one two three)\necho ${a[2]}\n", want: "three\n"},
		{name: "every element", script: "a=(one two three)\necho ${a[@]}\n", want: "one two three\n"},
		{name: "how many", script: "a=(one two three)\necho ${#a[@]}\n", want: "3\n"},
		{
			// The length of the element, not of the array -- the two are spelled
			// almost alike and mean different things.
			name: "the length of one element", script: "a=(one two)\necho ${#a[0]}\n", want: "3\n",
		},
		{name: "the subscripts", script: "a=(one two three)\necho ${!a[@]}\n", want: "0 1 2\n"},
		{
			// The reason arrays exist. Three elements, and the middle one keeps
			// its blank because it was quoted at assignment and quoted here.
			name:   "an element with a blank survives",
			script: "a=(one \"two words\" three)\nfor i in \"${a[@]}\"; do echo \"[$i]\"; done\n",
			want:   "[one]\n[two words]\n[three]\n",
		},
		{
			// Unquoted, the words split again -- which is what makes the quoted
			// form worth writing.
			name:   "unquoted splits again",
			script: "a=(one \"two words\" three)\nfor i in ${a[@]}; do printf '<%s>' $i; done\necho\n",
			want:   "<one><two><words><three>\n",
		},
		{
			name:   "star joins into one word",
			script: "a=(one \"two words\" three)\necho \"[${a[*]}]\"\n",
			want:   "[one two words three]\n",
		},
		{name: "assigning one element", script: "a=(one two)\na[1]=TWO\necho ${a[@]}\n", want: "one TWO\n"},
		{
			// Past the end leaves a gap rather than failing -- but the gap is not an
			// element. bash counts two here, not three, because an indexed array is
			// sparse. This case asserted three and its comment claimed bash did the
			// same; bash had not been asked. See array_sparse_test.go.
			name: "assigning past the end leaves a gap", script: "a=(one)\na[2]=three\necho ${#a[@]}\n", want: "2\n",
		},
		{
			// The gap is reachable and empty, which is what makes it a gap rather than
			// an absence.
			name: "the gap reads as empty", script: "a=(one)\na[2]=three\necho [${a[1]}]\n", want: "[]\n",
		},
		{name: "appending", script: "a=(one two)\na+=(three)\necho ${#a[@]}\n", want: "3\n"},
		{
			name:   "an empty array",
			script: "b=()\necho \"empty:[${b[@]}] count=${#b[@]}\"\n",
			want:   "empty:[] count=0\n",
		},
		{
			// A bare reference is element zero, which is bash's rule.
			name: "a bare reference is the first element", script: "a=(one two)\necho $a\n", want: "one\n",
		},
		{
			// Elements are expanded at assignment, so a parameter inside works.
			name:   "a parameter inside an assignment",
			script: "x=middle\na=(first $x last)\necho ${a[1]}\n",
			want:   "middle\n",
		},
		{
			// Out of range is empty rather than an error: a script testing
			// `${a[9]}` for emptiness is asking a reasonable question.
			name: "out of range is empty", script: "a=(one)\necho \"[${a[9]}]\"\n", want: "[]\n",
		},
		{
			// A scalar answers to a subscript, since every variable is an array of
			// one as far as `[0]` is concerned.
			name: "a scalar answers to [0]", script: "x=value\necho ${x[0]}\n", want: "value\n",
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
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// The parenthesis of an array assignment is part of a word, and four layers had
// to learn that -- the logical-line scanner, the group parser, the deferred scan
// and the lexer. Each of them refused it differently, so each is worth a case:
// nothing else about the shell's parentheses may have changed.
func TestRuntime_arrayAssignmentDoesNotDisturbParentheses(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a subshell still runs", script: "(echo inside)\n", want: "inside\n"},
		{name: "a command substitution still runs", script: "echo $(echo nested)\n", want: "nested\n"},
		{
			// An arithmetic expansion's parentheses are still its own.
			name: "arithmetic still evaluates", script: "echo $((1+2))\n", want: "3\n",
		},
		{
			// A function definition's parentheses are untouched.
			name: "a function can still be defined", script: "f() { echo called; }\nf\n", want: "called\n",
		},
		{
			// A subshell after an assignment on the same line is still a subshell:
			// the `(` there does not directly follow `name=`.
			name: "an assignment then a subshell", script: "x=1; (echo $x)\n", want: "1\n",
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
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
