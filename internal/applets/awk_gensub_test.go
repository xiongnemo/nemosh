package applets

import "testing"

// gensub is busybox's as well as gawk's, and was `call to undefined function` here. Every answer
// below is busybox-w32's (FRP-6075, measured from programs in files), and its awk_sub
// (editors/awk.c:2553) is the rule: the third argument is a number, so "g" and anything else
// that is not one is 0, every match, and N is the Nth alone; in the replacement `\N` is a group,
// `\0` and `&` the match, and a backslash before anything else stands for what follows it.
func TestAwkGensub_isBusyboxs(t *testing.T) {
	for _, test := range []struct{ program, input, want string }{
		{`BEGIN{print gensub(/a/,"b","g","aaa")}`, "", "bbb\n"},
		{`BEGIN{print gensub(/a/,"b","G","aaa")}`, "", "bbb\n"},
		{`BEGIN{print gensub(/a/,"b",2,"aaa")}`, "", "aba\n"},
		{`BEGIN{print gensub(/a/,"b","x","aaa")}`, "", "bbb\n"},
		{`BEGIN{print gensub(/a/,"b",0,"aaa")}`, "", "bbb\n"},
		{`BEGIN{print gensub(/a/,"b",5,"aaa")}`, "", "aaa\n"},
		{`BEGIN{print gensub(/(a)(b)/,"\\2\\1","g","abab")}`, "", "baba\n"},
		{`BEGIN{print gensub(/a+/,"[&]","g","xaay")}`, "", "x[aa]y\n"},
		{`BEGIN{print gensub(/a+/,"[\\0]","g","xaay")}`, "", "x[aa]y\n"},
		{`BEGIN{print gensub(/a+/,"[\\&]","g","xaay")}`, "", "x[&]y\n"},
		{`BEGIN{print gensub(/a+/,"[\\\\&]","g","xaay")}`, "", "x[\\aa]y\n"},
		{`BEGIN{print gensub(/(a)/,"\\9","g","xay")}`, "", "xy\n"},
		{`BEGIN{print gensub(/a/,"\\q","g","xay")}`, "", "xqy\n"},
		// The target is left as it was: gensub answers, where sub and gsub assign.
		{`BEGIN{s="aaa"; t=gensub(/a/,"b","g",s); print s, t}`, "", "aaa bbb\n"},
		{`{print gensub(/o/,"0","g")}`, "foo boo\n", "f00 b00\n"},
		{`BEGIN{print gensub("a.","X","g","aXabac")}`, "", "XXX\n"},
		{`BEGIN{print gensub(/x*/,"-","g","abc")}`, "", "-a-b-c-\n"},
		{`BEGIN{print gensub(/(é)/,"<\\1>","g","café")}`, "", "caf<é>\n"},
	} {
		if got, stderr, status := runAwk(t, test.program, test.input); got != test.want || status != 0 {
			t.Errorf("%s: %q (status %d, %s), want %q", test.program, got, status, stderr, test.want)
		}
	}
}

func TestAwkGensub_takesThreeOrFourArguments(t *testing.T) {
	for _, program := range []string{`BEGIN{print gensub(/a/,"b")}`, `BEGIN{print gensub(/a/,"b","g","x","y")}`} {
		if _, err := parseAwkProgram(program); err == nil {
			if _, stderr, status := runAwk(t, program, ""); status == 0 {
				t.Errorf("%s succeeded: %s", program, stderr)
			}
		}
	}
}
