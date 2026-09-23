package runtime_test

import "testing"

// `[[ -v name ]]` asks whether a name is set, empty or not. It was not an operator, so it read
// as a test of the string "-v" and was true whatever the name was. Every answer is bash's.
func TestDoubleBracketV_asksWhetherANameIsSet(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "set, and set empty", script: "x=1\ny=\n[[ -v x ]] && [[ -v y ]] && echo both\n", want: "both\n"},
		{name: "unset", script: "[[ -v nope ]] || echo unset\n", want: "unset\n"},
		{name: "safe under set -u", script: "set -u\n[[ -v nope ]] || echo safe\n", want: "safe\n"},
		{name: "an element, by arithmetic", script: "i=1\na=(p q)\n[[ -v a[i] ]] && ! [[ -v a[5] ]] && echo ok\n", want: "ok\n"},
		{name: "a bare array name is element zero", script: "a=([3]=z)\n[[ -v a ]] || echo not-zero\n", want: "not-zero\n"},
		{name: "a key", script: "k=k\ndeclare -A m=([k]=v)\n[[ -v m[$k] ]] && ! [[ -v m[j] ]] && echo ok\n", want: "ok\n"},
		{name: "a positional parameter", script: "set -- a\n[[ -v 1 ]] && ! [[ -v 2 ]] && echo ok\n", want: "ok\n"},
		{name: "a computed name", script: "[[ -v RANDOM ]] && echo ok\n", want: "ok\n"},
		{name: "in an expression", script: "x=1\n[[ -v x && ! -v nope ]] && echo ok\n", want: "ok\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// An unset element reads as empty. unset kept the old value in its slot and dropped only the
// mark, and the one-element read did not check the mark -- so the element came back.
func TestArray_anUnsetElementIsGone(t *testing.T) {
	stdout, _ := runScriptCapturing("a=(x y z)\nunset 'a[1]'\necho \"[${a[1]}] [${#a[1]}]\"\n")
	if stdout != "[] [0]\n" {
		t.Fatalf("stdout = %q, want the element gone", stdout)
	}
}
