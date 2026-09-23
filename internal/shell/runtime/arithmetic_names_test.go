package runtime_test

import "testing"

// What a name stands for in arithmetic. It read only stored variables and only plain numbers,
// so a computed name was 0 and a value that is itself an expression was 0; an element was a
// syntax error. The first three answers are busybox-w32's and bash's alike, the rest bash's.
func TestArithmetic_readsNamesAsBothReferencesDo(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "RANDOM is computed, not 0", script: "a=0\nfor i in 1 2 3 4 5 6 7 8; do [ $((RANDOM)) -ne 0 ] && a=$((a+1)); done\n[ $a -gt 0 ] && echo varies\n", want: "varies\n"},
		{name: "a value is an expression", script: "x=2+3\necho $((x*2))\n", want: "10\n"},
		{name: "a value is a name", script: "y=z\nz=7\necho $((y))\n", want: "7\n"},
		{name: "elements", script: "a=(1 2 3)\necho $(( a[0] + a[2] ))\n", want: "4\n"},
		{name: "an arithmetic subscript", script: "i=1\na=(x 9)\necho $(( a[i] * 2 ))\n", want: "18\n"},
		{name: "counting by key", script: "declare -A c\nfor w in a b a c a; do (( c[$w]++ )); done\necho \"${c[a]} ${c[b]}\"\n", want: "3 1\n"},
		{name: "a compound assignment to an element", script: "a=(5 6)\n(( a[1] += 10 ))\necho \"${a[@]}\"\n", want: "5 16\n"},
		{name: "prefix increment of an element", script: "a=(1 2)\n((++a[0]))\necho ${a[0]}\n", want: "2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A value that names itself is refused rather than followed forever, as bash refuses it.
func TestArithmetic_refusesAValueThatNamesItself(t *testing.T) {
	if stdout, status := runScriptCapturing("x=x\necho $((x))\necho after\n"); status == 0 || stdout != "" {
		t.Fatalf("got %q/%d, want a refusal", stdout, status)
	}
}
