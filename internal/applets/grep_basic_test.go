package applets_test

import "testing"

// A grep pattern is a POSIX basic expression unless -E, as in busybox, GNU and POSIX. It
// was read as extended, so `+ ? | ( ) { }` were operators and `\(a\)` matched nothing.
// Each answer is busybox-w32's, measured.
func TestGrep_readsABasicExpressionUnlessE(t *testing.T) {
	for _, test := range []struct {
		input string
		args  []string
		want  string
	}{
		{input: "a ()\nb\n", args: []string{"()"}, want: "a ()\n"},
		{input: "a ()\nb\n", args: []string{"a ("}, want: "a ()\n"},
		{input: "ab\nb\n", args: []string{`\(a\)b`}, want: "ab\n"},
		{input: "a+b\naab\n", args: []string{"a+b"}, want: "a+b\n"},
		{input: "a{2}\naa\n", args: []string{"a{2}"}, want: "a{2}\n"},
		{input: "a|b\nb\n", args: []string{"a|b"}, want: "a|b\n"},
		{input: "aab\nab\n", args: []string{`a\{2\}`}, want: "aab\n"},
		{input: "ab\n", args: []string{`a\|x`}, want: "ab\n"},
		{input: "aab\n", args: []string{"-o", `a\+b`}, want: "aab\n"},
		{input: "foo bar\n", args: []string{"-o", `\<bar`}, want: "bar\n"},
		{input: "*a\n", args: []string{"*a"}, want: "*a\n"},
		{input: "x1\nx\n", args: []string{"-E", "x[0-9]+"}, want: "x1\n"},
		{input: "ab\n", args: []string{"-E", "a(b)"}, want: "ab\n"},
		{input: "a+b\naab\n", args: []string{"-G", "a+b"}, want: "a+b\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "grep", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("grep %q = %q (err %v), want %q", test.args, stdout, err, test.want)
		}
	}
}

// GNU's classes and word edges, in sed as in grep; `\w` matched a literal w.
func TestSed_readsGNUClassesAndWordEdges(t *testing.T) {
	for _, test := range []struct{ input, script, want string }{
		{input: "a w\n", script: `s/\w/X/`, want: "X w\n"},
		{input: "foo bar\n", script: `s/\bbar/X/`, want: "foo X\n"},
		{input: "foo bar\n", script: `s/\<b/X/`, want: "foo Xar\n"},
		{input: "foo bar\n", script: `s/o\>/X/`, want: "foX bar\n"},
		{input: "a b\n", script: `s/\s/_/`, want: "a_b\n"},
		{input: "a.b\n", script: `s/\W/_/`, want: "a_b\n"},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "sed", test.script)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q = %q (err %v), want %q", test.script, stdout, err, test.want)
		}
	}
}
