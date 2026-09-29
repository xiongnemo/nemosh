package applets_test

import (
	"testing"
)

// basename takes -s SUFFIX, which implies -a, and refuses a third operand without either, as
// busybox's does; dirname takes one NAME. -s was refused, and `basename a b c` and `dirname a b`
// printed an answer for the first and dropped the rest.
func TestBasename_takesASuffixOptionAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args    []string
		want    string
		wantErr string
	}{
		{[]string{"basename", "-s", ".c", "/a/b.c"}, "b\n", ""},
		{[]string{"basename", "-s", ".c", "-a", "x.c", "y.c"}, "x\ny\n", ""},
		{[]string{"basename", "-sfoo", "xfoo"}, "x\n", ""},
		{[]string{"basename", ".c", ".c"}, ".c\n", ""},
		{[]string{"basename", ""}, "\n", ""},
		{[]string{"basename", "a", "b", "c"}, "", "extra operand 'c'"},
		{[]string{"dirname", "a/b", "c/d"}, "", "extra operand 'c/d'"},
	} {
		stdout, _, err := runPermuted(t, view, "", test.args...)
		if stdout != test.want || (err == nil) != (test.wantErr == "") || err != nil && err.Error() != test.wantErr {
			t.Errorf("%q: got %q, %v; want %q, %q", test.args, stdout, err, test.want, test.wantErr)
		}
	}
}
