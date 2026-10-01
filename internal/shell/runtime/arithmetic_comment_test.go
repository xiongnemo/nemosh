package runtime

import (
	"strings"
	"testing"
)

// A # in arithmetic is the expression's, on a later line too, and no comment: both references
// refuse `$((` then `1 + 2  # x` then `))`, and bash, which has (( )), refuses it there. The
// scan took it for a comment, and they answered 3 and set a to 7. A command substitution in the
// expression is a script again, with comments of its own.
func TestArithmetic_aHashOnALaterLineIsNoComment(t *testing.T) {
	for _, test := range []struct{ script, stdout, stderr string }{
		{script: "echo $((\n1 + 2  # not a comment\n))\necho after\n", stderr: `unexpected "#"`},
		{script: "(( a = 3 + 4  # comment\n))\necho \"[$a]\"\n", stdout: "[]\n", stderr: `unexpected "#"`},
		{script: "echo $(( $(echo 2 # two\n) + 1 ))\n", stdout: "3\n"},
		{script: "echo $((\n1 +\n2 + \\\n3\n))\n", stdout: "6\n"},
	} {
		stdout, stderr, _ := runKill(t, test.script)
		if stdout != test.stdout || test.stderr == "" && stderr != "" || !strings.Contains(stderr, test.stderr) {
			t.Errorf("%q: stdout %q, stderr %q; want %q and a refusal naming %q", test.script, stdout, stderr, test.stdout, test.stderr)
		}
	}
}

// A character no operator is refuses the expression before an assignment it follows is made, as
// both references refuse it. A stray `)`, which ends a whole expression, comes after the
// assignment in both, and here.
func TestArithmetic_aStrayCharacterRefusesBeforeTheAssignment(t *testing.T) {
	for script, want := range map[string]string{
		"let 'a = 3 + 4 # x'\necho \"a=$a\"\n":       "a=\n",
		"let 'a = 3 + 4 )'\necho \"a=$a\"\n":         "a=7\n",
		"let 'a = 5, b = 1 +'\necho \"a=$a b=$b\"\n": "a=5 b=\n",
		"echo $(( 2#101 + 16#ff ))\n":                "260\n",
	} {
		if stdout, _, _ := runKill(t, script); stdout != want {
			t.Errorf("%q: stdout %q, want %q", script, stdout, want)
		}
	}
}
