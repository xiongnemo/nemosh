package runtime_test

import "testing"

// A `$(...)` inside an expansion or an expression ran nowhere.
//
//	$(( $(echo 2) * 3 ))   arithmetic syntax error: unexpected "$"
//	${x:-$(echo sub)}      printed the seven characters $(echo sub)
//	${a[$(echo 1)]}        empty
//
// The second is the one that matters most: `${TMPDIR:-$(mktemp -d)}` is an ordinary
// line, and printing the text at the reader is a silent wrong answer.
//
// All three now go through the same walk, and the substitution itself runs through the
// same path a `$( )` in an ordinary word takes -- so the two cannot disagree about what
// `$(echo a; echo b)` produces.
func TestEmbeddedSubstitution_runsInsideExpansionsAndArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "inside arithmetic", script: "echo $(( $(echo 2) * 3 ))\n", want: "6\n"},
		{
			name: "inside arithmetic with a variable too", script: "i=4\necho $(( $(echo 2) + $i ))\n", want: "6\n",
		},
		{name: "as a default value", script: "echo \"${x:-$(echo sub)}\"\n", want: "sub\n"},
		{name: "as a default in the backquote spelling", script: "echo \"${x:-`echo bq`}\"\n", want: "bq\n"},
		{
			// Two lines of output keep their newline, as they do in an ordinary word.
			name: "a default of two lines", script: "echo \"${x:-$(echo a; echo b)}\"\n", want: "a\nb\n",
		},
		{name: "as an assign-default", script: "echo \"${x:=$(echo v)}\"\necho \"$x\"\n", want: "v\nv\n"},
		{name: "as an alternative value", script: "x=1\necho \"${x:+$(echo yes)}\"\n", want: "yes\n"},
		{name: "in a subscript", script: "a=(p q)\necho \"${a[$(echo 1)]}\"\n", want: "q\n"},
		{name: "a length in a subscript", script: "a=(p q r)\nx=ab\necho \"${a[${#x}]}\"\n", want: "r\n"},
		{name: "in an assignment subscript", script: "a=(x)\na[$(echo 0)]=Z\necho \"${a[0]}\"\n", want: "Z\n"},
		{name: "in a for loop condition", script: "for ((i=0;i<$(echo 2);i++)); do printf %s $i; done\necho\n", want: "01\n"},
		{
			// Not run when the value is set: a default is not evaluated unless it is
			// needed, which matters when the command has an effect.
			name: "not run when the parameter is set", script: "x=1\necho \"${x:-$(echo ran)}\"\n", want: "1\n",
		},
		// The forms that already worked, so the walk cannot cost them.
		{name: "plain arithmetic", script: "echo $(( 1 + 1 ))\n", want: "2\n"},
		{name: "a plain default", script: "echo \"${x:-plain}\"\n", want: "plain\n"},
		{name: "a nested reference", script: "y=v\necho \"${x:-${y}}\"\n", want: "v\n"},
		{name: "a substitution in an ordinary word", script: "echo $(echo word)\n", want: "word\n"},
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

// A default whose command is not needed must not run: the substitution is evaluated
// only where the value is taken from it. Checked by having the command leave a trace.
func TestEmbeddedSubstitution_isNotRunWhenItIsNotNeeded(t *testing.T) {
	// When -- `marker` would print if the default were evaluated.
	status, stdout, stderr := runSetScript(t, "x=set\necho \"${x:-$(echo marker >&2)}\"\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "set\n" {
		t.Fatalf("stdout = %q, want the set value", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want nothing -- the default was evaluated when it was not needed", stderr)
	}
}
