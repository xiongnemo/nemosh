package runtime_test

import "testing"

// `${VAR:-${DEFAULT}}` printed `}`.
//
// One of the commonest lines in any shell script, and it produced a stray brace. Two
// causes, both silent:
//
//   - the scan that finds the end of a `${...}` took the *first* `}` in the rest of
//     the text, so the reference was cut at `${x:-${y` and the trailing brace became
//     literal;
//   - the operand was then looked up as a whole variable name, so `${y}` asked for a
//     variable called `{y}` and found nothing.
//
// Both are fixed, and every expectation below was measured against bash.
func TestParameterNesting_expandsAReferenceInsideAReference(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a nested default", script: "y=v\necho \"${x:-${y}}\"\n", want: "v\n"},
		{name: "a nested default two deep", script: "echo \"${x:-${y:-deep}}\"\n", want: "deep\n"},
		{name: "a nested assign-default", script: "y=v\necho \"${x:=${y}}\"\n", want: "v\n"},
		{name: "a nested alternative", script: "x=1\necho \"${x:+set${x}}\"\n", want: "set1\n"},
		{name: "a nested length", script: "x=abc\necho \"${nope:-${#x}}\"\n", want: "3\n"},
		{
			// A reference that is not at the start of the operand: the operand path
			// returned the text unchanged when it did not begin with a dollar.
			name: "a reference in the middle", script: "y=v\necho \"${x:-pre${y}post}\"\n", want: "prevpost\n",
		},
		{name: "text around a nested default", script: "echo \"${x:-a${y:-b}c}\"\n", want: "abc\n"},
		{name: "a nested reference in a subscript", script: "a=(p q)\ni=1\necho \"${a[${i}]}\"\n", want: "q\n"},
		{
			name:   "a nested reference in an associative key",
			script: "declare -A m\nm[k]=v\nk=k\necho \"${m[${k}]}\"\n", want: "v\n",
		},
		{
			// The `}` is data here, so it must not end the expansion.
			name: "a brace inside quotes does not close it", script: "y=v\necho \"${x:-${y}}}\"\n", want: "v}\n",
		},
		// The forms that were already right, so the depth counting cannot cost them.
		{name: "a plain default", script: "echo \"${x:-plain}\"\n", want: "plain\n"},
		{name: "a set value", script: "x=1\necho \"${x:-no}\"\n", want: "1\n"},
		{name: "an empty default", script: "echo \"[${x:-}]\"\n", want: "[]\n"},
		{name: "a bare braced name", script: "x=v\necho \"${x}\"\n", want: "v\n"},
		{name: "a braced positional", script: "set -- p\necho \"${1}\"\n", want: "p\n"},
		{name: "a braced status", script: "true\necho \"${?}\"\n", want: "0\n"},
		{name: "a suffix strip", script: "x=a.b\necho \"${x%.*}\"\n", want: "a\n"},
		{name: "a length", script: "x=abc\necho \"${#x}\"\n", want: "3\n"},
		{name: "two references in one word", script: "a=1\nb=2\necho \"${a}${b}\"\n", want: "12\n"},
		{name: "a reference beside literal text", script: "a=1\necho \"x${a}y\"\n", want: "x1y\n"},
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

// The message of `${x?word}` is an operand like any other, so a reference in it has to
// expand -- otherwise the diagnostic tells the reader `${y}` rather than what y held.
func TestParameterNesting_expandsTheMessageOfTheQuestionOperator(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "y=needed\necho \"${x?${y}}\"\n")

	// Then
	if want := "needed"; !contains(stderr, want) {
		t.Fatalf("stderr = %q, want it to contain %q rather than the reference itself", stderr, want)
	}
}

func contains(text, want string) bool {
	return len(want) == 0 || len(text) >= len(want) && indexOf(text, want) >= 0
}

func indexOf(text, want string) int {
	for index := 0; index+len(want) <= len(text); index++ {
		if text[index:index+len(want)] == want {
			return index
		}
	}
	return -1
}
