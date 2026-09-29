package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// uniq takes busybox's -f -s -w -z and an OUTPUT operand, each case measured there: -f passes
// fields, each its blanks and what follows up to the next blank, -s characters, -w compares at
// most N of what is left, -i folds ASCII alone, and -z ends each line with NUL. It took -c -d -u
// -i, refused a second operand, and folded case beyond ASCII.
func TestUniq_answersAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{[]string{"-f1"}, "a 1\nb 1\nc 2\n", "a 1\nc 2\n"},
		{[]string{"-s1"}, "xa\nya\nzb\n", "xa\nzb\n"},
		{[]string{"-w2"}, "ab1\nab2\nac\n", "ab1\nac\n"},
		{[]string{"-i"}, "A\na\n", "A\n"},
		{[]string{"-i"}, "\xc3\x89\n\xc3\xa9\n", "\xc3\x89\n\xc3\xa9\n"},
		{[]string{"-z"}, "a\na\n", "a\x00"},
		{[]string{"-c"}, "a\na\nb\n", "      2 a\n      1 b\n"},
		{[]string{"-cd"}, "a\na\nb\n", "      2 a\n"},
		{[]string{"-du"}, "a\na\nb\n", ""},
		// The next field's blanks stay in what is compared.
		{[]string{"-f1"}, "a  b\na b\n", "a  b\na b\n"},
		{[]string{"-s9"}, "ab\ncd\n", "ab\n"},
		{[]string{"-w0"}, "ab\ncd\n", "ab\n"},
		{[]string{}, "a\na", "a\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.input, append([]string{"uniq"}, test.args...)...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("uniq %q on %q: got %q, %q, %v; want %q", test.args, test.input, stdout, stderr, err, test.want)
		}
	}
	for _, args := range [][]string{{"-f", "x"}, {"-f", "-1"}} {
		if _, stderr, err := runPermuted(t, view, "a\n", append([]string{"uniq"}, args...)...); err == nil || stderr != "uniq: invalid number '"+args[1]+"'\n" {
			t.Errorf("uniq %q: got %q, %v; want invalid number", args, stderr, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "in"), []byte("a\na\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, _, err := runPermuted(t, view, "", "uniq", "in", "out"); stdout != "" || err != nil {
		t.Fatalf("uniq in out: %q, %v", stdout, err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "out")); err != nil || string(data) != "a\nb\n" {
		t.Errorf("out holds %q, %v; want a and b", data, err)
	}
}
