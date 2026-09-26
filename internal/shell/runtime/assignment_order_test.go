package runtime_test

import "testing"

// Assignments are made in order, and each one's value sees the assignments before it: `a=1
// b=$a` gives b the 1. They were all expanded first and made afterwards, so b was empty. A
// command's arguments are expanded before any of them and see none, as POSIX 2.9.1 orders
// it. busybox-w32 and bash 5.3 agree on every answer here but `+=`, which busybox does not
// have and bash appends with.
func TestRuntime_assignmentsSeeTheOnesBefore(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=1 b=$a; echo "[$b]"`, "[1]\n"},
		{`x=5; x=6 y=$x; echo "[$y]"`, "[6]\n"},
		{`x=1 x=2 y=$x; echo "$x $y"`, "2 2\n"},
		{`u=1 v=$(echo $u); echo "$v"`, "1\n"},
		{`p=1 q=$p env | grep '^q='`, "q=1\n"},
		{`FOO="foo" BAR="[$FOO][$BAZ]" BAZ=baz env | grep -E '^(FOO|BAR|BAZ)=' | sort`, "BAR=[foo][]\nBAZ=baz\nFOO=foo\n"},
		{`p=; f() { echo "in f: [$1] [$p]"; }; p=7 r=$p f "$p"; echo "after: [$p]"`, "in f: [] [7]\nafter: []\n"},
		{`c=1 d=$c echo "[$c][$d]"; echo "[$c][$d]"`, "[][]\n[][]\n"},
		{`a=x a+=y b=$a; echo "$a $b"`, "xy xy\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
