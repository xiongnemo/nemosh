package applets_test

import (
	"testing"
)

// -d's list is read for escapes as busybox reads it: \t a tab, \n a newline, \\ a backslash,
// and \0 a delimiter that is nothing; a character that is no escape keeps its backslash. Each
// answer is busybox-w32's, measured: the list was taken as written.
func TestPaste_readsTheEscapesOfMinusD(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a": "1\n2\n", "b": "x\ny\n", "c": "p\nq\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-d", `\t,`, "a", "b", "c"}, want: "1\tx,p\n2\ty,q\n"},
		{args: []string{"-d", `\0`, "a", "b"}, want: "1x\n2y\n"},
		{args: []string{"-d", `\\`, "a", "b"}, want: "1\\x\n2\\y\n"},
		{args: []string{"-d", `\n`, "a", "b"}, want: "1\nx\n2\ny\n"},
		{args: []string{"-d", `\z`, "a", "b", "c"}, want: "1\\xzp\n2\\yzq\n"},
		{args: []string{"-s", "-d", `\0,`, "a", "c"}, want: "12\npq\n"},
	} {
		if got, stderr, err := runSmall(t, dir, "", "paste", test.args...); got != test.want || err != nil {
			t.Errorf("paste %q = %q, %v (%s); want %q", test.args, got, err, stderr, test.want)
		}
	}
}
