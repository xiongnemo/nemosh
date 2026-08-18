package runtime_test

import (
	"strings"
	"testing"
)

// The bash parameter expansions, all measured against bash on the machine this was
// written on. Each of these used to be `bad substitution`, which stops a script at
// its first line rather than at the feature it needed.
func TestParameterTransform_matchesBash(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		// Substring. The offsets are arithmetic expressions, not literals.
		{name: "substring with a length", script: "x=abcdef\necho ${x:1:3}\n", want: "bcd\n"},
		{name: "substring to the end", script: "x=abcdef\necho ${x:3}\n", want: "def\n"},
		{name: "substring of one", script: "x=abcdef\necho ${x:0:1}\n", want: "a\n"},
		{
			// The space is required, and it is what tells this apart from `:-`.
			name: "a negative offset counts from the end", script: "x=abcdef\necho ${x: -2}\n", want: "ef\n",
		},
		{
			// A negative length stops that many short of the end rather than
			// counting forward.
			name: "a negative length stops short of the end", script: "x=abcdef\necho ${x:1:-1}\n", want: "bcde\n",
		},
		{name: "an offset past the end is empty", script: "x=abcdef\necho [${x:9}]\n", want: "[]\n"},
		{name: "a length past the end is the rest", script: "x=abcdef\necho ${x:0:99}\n", want: "abcdef\n"},
		{name: "the offset can be an expression", script: "x=abcdef\nn=2\necho ${x:n:2}\n", want: "cd\n"},
		{name: "the offset can be arithmetic", script: "x=abcdef\necho ${x:1+1:2}\n", want: "cd\n"},
		{
			// Characters, not bytes: counting bytes would cut this mid-rune.
			name: "substring counts characters", script: "x=áéíóú\necho ${x:1:2}\n", want: "éí\n",
		},
		{name: "substring of a positional parameter", script: "set -- hello\necho ${1:1:3}\n", want: "ell\n"},

		// Replacement.
		{name: "replace the first match", script: "x=aXbXc\necho ${x/X/-}\n", want: "a-bXc\n"},
		{name: "replace every match", script: "x=aXbXc\necho ${x//X/-}\n", want: "a-b-c\n"},
		{name: "replace anchored at the start", script: "x=aXbXc\necho ${x/#a/A}\n", want: "AXbXc\n"},
		{name: "replace anchored at the end", script: "x=aXbXc\necho ${x/%c/C}\n", want: "aXbXC\n"},
		{name: "an anchored pattern that is not there", script: "x=aXbXc\necho ${x/#X/-}\n", want: "aXbXc\n"},
		{name: "a missing replacement deletes", script: "x=aXbXc\necho ${x//X}\n", want: "abc\n"},
		{name: "no match leaves the value alone", script: "x=aXbXc\necho ${x/nope/z}\n", want: "aXbXc\n"},
		{
			// A shell pattern, not a regular expression.
			name: "a bracket pattern", script: "x=a1b2c\necho ${x//[0-9]/#}\n", want: "a#b#c\n",
		},
		{
			// Greedy: `b*` takes everything it can, which is what bash does.
			name: "a star pattern is greedy", script: "x=a1b2c\necho ${x/b*/-}\n", want: "a1-\n",
		},
		{name: "replacing in an empty value", script: "x=\necho [${x//a/b}]\n", want: "[]\n"},
		{name: "a question mark pattern", script: "x=abc\necho ${x/?/-}\n", want: "-bc\n"},

		// Case conversion.
		{name: "upper every character", script: "x=aBc\necho ${x^^}\n", want: "ABC\n"},
		{name: "lower every character", script: "x=aBc\necho ${x,,}\n", want: "abc\n"},
		{name: "upper the first character", script: "x=aBc\necho ${x^}\n", want: "ABc\n"},
		{name: "lower the first character", script: "x=ABc\necho ${x,}\n", want: "aBc\n"},
		{
			// A pattern narrows which characters are touched.
			name: "upper only what the pattern selects", script: "x=abc\necho ${x^^[ab]}\n", want: "ABc\n",
		},
		{name: "case conversion of an empty value", script: "x=\necho [${x^^}]\n", want: "[]\n"},

		// Indirection.
		{name: "indirect through a name", script: "a=b\nb=c\necho ${!a}\n", want: "c\n"},
		{name: "indirect through an unset name is empty", script: "a=nope\necho [${!a}]\n", want: "[]\n"},
		{name: "indirect through an empty value is empty", script: "a=\necho [${!a}]\n", want: "[]\n"},
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

// The new `:` operator sits next to four that begin with a colon, and `/` next to
// nothing at all. These are the spellings that would break if the operator table
// were ordered wrongly -- and `${x:-2}` against `${x: -2}` is a real pair of
// meanings, not a contrived one.
func TestParameterTransform_doesNotDisturbTheOperatorsBesideIt(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "default for unset", script: "echo ${u:-fallback}\n", want: "fallback\n"},
		{name: "default without the colon", script: "echo ${u-bare}\n", want: "bare\n"},
		{
			// No space: still the default operator, and the word is `2`.
			name: "a colon-dash with a number is still a default", script: "echo ${u:-2}\n", want: "2\n",
		},
		{
			// A space: a substring with a negative offset.
			name: "a colon space minus is a substring", script: "x=abcdef\necho ${x: -2}\n", want: "ef\n",
		},
		{name: "assign if unset", script: "echo ${u:=assigned}\necho $u\n", want: "assigned\nassigned\n"},
		{name: "alternative when set", script: "x=1\necho ${x:+yes}\n", want: "yes\n"},
		{name: "longest prefix strip", script: "x=path/to/file\necho ${x##*/}\n", want: "file\n"},
		{name: "shortest prefix strip", script: "x=path/to/file\necho ${x#*/}\n", want: "to/file\n"},
		{name: "longest suffix strip", script: "x=a.b.c\necho ${x%%.*}\n", want: "a\n"},
		{name: "shortest suffix strip", script: "x=a.b.c\necho ${x%.*}\n", want: "a.b\n"},
		{name: "length", script: "x=abc\necho ${#x}\n", want: "3\n"},
		{name: "a bare braced name", script: "x=v\necho ${x}\n", want: "v\n"},
		{name: "a positional parameter", script: "set -- one two\necho ${2}\n", want: "two\n"},
		{name: "the status", script: "true\necho ${?}\n", want: "0\n"},
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

// The array subscript forms must keep winning over the new `!` prefix: `${!a[@]}` is
// a list of subscripts, not indirection through a name called `a[@]`.
func TestParameterTransform_leavesTheArrayFormsAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "subscripts", script: "a=(x y z)\necho ${!a[@]}\n", want: "0 1 2\n"},
		{name: "count", script: "a=(x y z)\necho ${#a[@]}\n", want: "3\n"},
		{name: "an element", script: "a=(x y z)\necho ${a[1]}\n", want: "y\n"},
		{name: "every element", script: "a=(x y z)\necho ${a[@]}\n", want: "x y z\n"},
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

// An offset is an arithmetic expression, so an unset name in one is zero rather
// than an error -- measured, `${x:notanumber:2}` over abcdef gives `ab` in bash,
// because arithmetic reads an unset variable as 0. Only malformed arithmetic fails.
func TestParameterTransform_treatsAnUnsetOffsetNameAsZero(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "x=abcdef\necho [${x:notanumber:2}]\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "[ab]\n" {
		t.Fatalf("stdout = %q, want [ab] as bash gives", stdout)
	}
}

func TestParameterTransform_refusesMalformedArithmeticInAnOffset(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "x=abcdef\necho ${x:1+:2}\n")

	// Then
	if status == 0 {
		t.Fatalf("status = 0, want a failure; stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "bad substitution") {
		t.Fatalf("stderr = %q, want it to name a bad substitution", stderr)
	}
}
