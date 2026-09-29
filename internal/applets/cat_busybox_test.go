package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// cat takes busybox's letters, each case measured there: -v -e -t -A show what does not print as
// catv does, numbering there with two spaces; -n and -b number whole lines with a tab, putting
// back a last line's newline and, as busybox-w32's text mode does, dropping the carriage return
// before one; -u is taken and ignored. It took -n alone.
func TestCat_answersAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{[]string{"-b"}, "a\n\nb\n", "     1\ta\n\n     2\tb\n"},
		{[]string{"-A"}, "a\tb\r\n\x01\x7f\x80\xff\n", "a^Ib^M$\n^A^?M-^@M-^?$\n"},
		{[]string{"-v"}, "a\tb\n", "a\tb\n"},
		{[]string{"-t"}, "a\tb\n", "a^Ib\n"},
		{[]string{"-e"}, "a\n", "a$\n"},
		{[]string{"-et"}, "a\tb\n", "a^Ib$\n"},
		{[]string{"-v"}, "\xc3\xa9\n", "M-CM-)\n"},
		{[]string{"-n"}, "a\nb", "     1\ta\n     2\tb\n"},
		{[]string{"-n"}, "a\r\nb\r\n", "     1\ta\n     2\tb\n"},
		{[]string{"-v"}, "a\r\n", "a^M\n"},
		{[]string{}, "a\r\n", "a\r\n"},
		{[]string{"-nv"}, "a\nb", "     1  a\n     2  b"},
		{[]string{"-bv"}, "a\n\nb\n", "     1  a\n\n     2  b\n"},
		{[]string{"-nb"}, "a\n\nb\n", "     1\ta\n\n     2\tb\n"},
		{[]string{"-u"}, "a\n", "a\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.input, append([]string{"cat"}, test.args...)...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("cat %q on %q: got %q, %q, %v; want %q", test.args, test.input, stdout, stderr, err, test.want)
		}
	}
	// The count, and catv's place in a line, run on from one operand into the next.
	if err := os.WriteFile(filepath.Join(dir, "g"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-n", "g", "g"}, "     1\tx\n     2\tx\n"},
		{[]string{"-nv", "g", "g"}, "     1  xx"},
	} {
		if stdout, _, err := runPermuted(t, view, "", append([]string{"cat"}, test.args...)...); stdout != test.want || err != nil {
			t.Errorf("cat %q: got %q, %v; want %q", test.args, stdout, err, test.want)
		}
	}
	if _, _, err := runPermuted(t, view, "", "cat", "-z"); err == nil {
		t.Error("cat -z succeeded; busybox has no -z")
	}
}
