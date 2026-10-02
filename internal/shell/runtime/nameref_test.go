package runtime_test

import (
	"strings"
	"testing"
)

// Namerefs, bash's: `declare -n ref=name` makes ref another name for name, to read, to write
// and to unset through. busybox has none; `local -n` was "not an option this build has", so a
// function taking an array by name could not be written. Each answer is bash 5.3's.
func TestNameref(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{
			name:   "an array passed by name",
			script: "show() { local -n a=$1; echo \"${a[$2]} ${#a[@]}\"; }\nshadock=(ga bu zo meu)\nshow shadock 2\n",
			stdout: "zo 4\n",
		},
		{
			name:   "an array changed by name",
			script: "set1() { local -n a=$1; a[1]=$2; }\nshadock=(a b c d)\nset1 shadock ZZZ\necho ${shadock[@]}\n",
			stdout: "a ZZZ c d\n",
		},
		{
			name:   "an associative array by name",
			script: "declare -A days=([monday]=eggs [sunday]=jam)\nshow() { local -n m=$1; local k=$2; echo \"${m[$k]}\"; }\nshow days sunday\n",
			stdout: "jam\n",
		},
		{
			name:   "a caller's local, by dynamic scope",
			script: "f3() { local -n ref=$1; ref=x; }\nf2() { f3 \"$@\"; }\nf1() { local F1=F1; f2 F1; echo \"F1=$F1\"; }\nf1\n",
			stdout: "F1=x\n",
		},
		{
			name:   "-n and +n",
			script: "x=foo\nref=x\necho \"$ref\"\ntypeset -n ref\necho \"$ref\"\nx=bar\necho \"$ref\"\ntypeset +n ref\necho \"$ref\"\n",
			stdout: "x\nfoo\nbar\nx\n",
		},
		{
			name:   "a write goes to the target",
			script: "y=YY\ntypeset -n ref=y\nref=XXXX\necho \"$ref $y\"\n",
			stdout: "XXXX XXXX\n",
		},
		{
			// bash inverts it: through a nameref, ${!ref} is the name it leads to.
			name:   "${!ref} is the target's name",
			script: "foo=FOO\nx=foo\ntypeset -n ref=x\necho \"$ref ${!ref}\"\n",
			stdout: "foo x\n",
		},
		{
			name:   "an empty nameref takes what it is given as its target",
			script: "typeset -n ref\necho \"[$ref]\"\nref=x\nx=XX\necho \"[$ref]\"\ndeclare -p ref\n",
			stdout: "[]\n[XX]\ndeclare -n ref=\"x\"\n",
		},
		{
			name:   "unset goes to the target, unset -n to the nameref",
			script: "x=X\ntypeset -n ref=x\nunset ref\necho \"[$ref] [${x-unset}]\"\nx=Y\nunset -n ref\necho \"[${ref-unset}] [$x]\"\n",
			stdout: "[] [unset]\n[unset] [Y]\n",
		},
		{
			name:   "a chain",
			script: "x=foo\ntypeset -n ref=x\ntypeset -n ref2=ref\necho \"$ref2\"\n",
			stdout: "foo\n",
		},
		{
			name:   "an element",
			script: "typeset -n ref='a[2]'\na=(zero one two three)\necho \"$ref\"\n",
			stdout: "two\n",
		},
		{
			name:   "not a name, not a nameref",
			script: "ref=1\ntypeset -n ref 2>/dev/null\necho \"st=$? $ref\"\nref=foo\necho \"$ref\"\n",
			stdout: "st=1 1\nfoo\n",
		},
		{
			name:   "arithmetic follows it",
			script: "n=4\ntypeset -n ref=n\necho $(( ref + 1 ))\n(( ref += 10 ))\necho \"$n\"\n",
			stdout: "5\n14\n",
		},
		{
			name:   "a read-only target cannot be written through",
			script: "x=X\ntypeset -n ref=x\nreadonly x\n(ref=XX) 2>/dev/null\necho \"st=$? $x\"\n",
			stdout: "st=2 X\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}

// A nameref to an element takes no subscript of its own: `ref[0]=x` is no identifier, and the
// statement is abandoned with the rest of its line, status 1, as bash abandons it. It went
// on, status 0. Measured against bash 5.3.
func TestNameref_aSubscriptOnANamerefToAnElementAbandonsTheLine(t *testing.T) {
	script := "array=(X Y Z)\ntypeset -n ref='array[0]'\nref[0]=foo; echo same\necho status=$?\necho ${array[@]}\n"
	status, stdout, stderr := runSetScript(t, script)
	if stdout != "status=1\nX Y Z\n" || status != 0 || !strings.Contains(stderr, "`array[0]': not a valid identifier") {
		t.Fatalf("got %q/%d, stderr %q", stdout, status, stderr)
	}
}
