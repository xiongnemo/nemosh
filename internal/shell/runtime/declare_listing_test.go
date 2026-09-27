package runtime_test

import "testing"

// `declare` with no names lists as bash lists: bare, `x='a b'`, with the functions after, and
// with -p or attributes as declarations, the ones with any attribute asked for. The answers
// are bash 5.3's; busybox has no declare.
func TestRuntime_declareListsAsBashDoes(t *testing.T) {
	const given = "f() { echo hi; }; x='a b'; y=plain; z=; q=\"it's\"; t=$'a\\tb'; arr=(1 '2 3'); declare -A m=([k]=v)\n" +
		"declare -i n=5; declare -r r=1; declare -ir b=2; declare -a ea; declare -i u\n"
	tests := []struct {
		script, want string
	}{
		{
			given + "declare | grep -E '^(x|y|z|q|t|arr|m|n|r|ea|u)='",
			"arr=([0]=\"1\" [1]=\"2 3\")\nm=([k]=\"v\" )\nn=5\nq='it'\\''s'\nr=1\nt=$'a\\tb'\nx='a b'\ny=plain\nz=\n",
		},
		{given + "declare | grep -A3 '^f ()'", "f () \n{ \n    echo hi\n}\n"},
		{given + "declare -i | grep -E ' (n|u)(=|$)'", "declare -i n=\"5\"\ndeclare -i u\n"},
		{given + "declare -ir | grep -E ' (n|r|b)='", "declare -ir b=\"2\"\ndeclare -i n=\"5\"\ndeclare -r r=\"1\"\n"},
		{given + "declare -a | grep ' arr='; declare -A | grep ' m='", "declare -a arr=([0]=\"1\" [1]=\"2 3\")\ndeclare -A m=([k]=\"v\" )\n"},
		{given + "declare -p | grep -E '^declare -. (arr|m|x)='", "declare -a arr=([0]=\"1\" [1]=\"2 3\")\ndeclare -A m=([k]=\"v\" )\ndeclare -- x=\"a b\"\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
