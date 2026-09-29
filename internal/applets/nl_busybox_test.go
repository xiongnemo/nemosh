package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nl numbers as busybox's does, each case measured against busybox-w32: a line of blanks is
// not empty, a line not numbered has as many blanks as the number and the separator took, and
// -i -s -v -w and their long forms say how. It took -b alone and left a line of blanks
// unnumbered.
func TestNl_numbersAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	const input = "a\n\n   \nb\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "     1\ta\n       \n     2\t   \n     3\tb\n"},
		{[]string{"-ba"}, "     1\ta\n     2\t\n     3\t   \n     4\tb\n"},
		{[]string{"-bn"}, "       a\n       \n          \n       b\n"},
		{[]string{"-bnone"}, "       a\n       \n          \n       b\n"},
		{[]string{"-i", "3", "-v", "0"}, "     0\ta\n       \n     3\t   \n     6\tb\n"},
		{[]string{"-s:", "-w", "2"}, " 1:a\n   \n 2:   \n 3:b\n"},
		{[]string{"-w", "0"}, "1\ta\n \n2\t   \n3\tb\n"},
		{[]string{"-s", "", "-w", "1"}, "1a\n \n2   \n3b\n"},
		{[]string{"-p"}, "     1\ta\n       \n     2\t   \n     3\tb\n"},
		{[]string{"--number-width=3", "--number-separator=,", "--starting-line-number=7", "--line-increment=2", "--body-numbering=a"},
			"  7,a\n  9,\n 11,   \n 13,b\n"},
		// pBRE is GNU's: the lines BRE matches. busybox numbers none.
		{[]string{"-bp^[ab]"}, "     1\ta\n       \n          \n     2\tb\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, input, append([]string{"nl"}, test.args...)...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("nl %q: %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-v", "-2"}, "invalid number '-2'"},
		{[]string{"-w", "+3"}, "invalid number '+3'"},
		{[]string{"-i", "x"}, "invalid number 'x'"},
		{[]string{"-w", "3000000000"}, "number 3000000000 is not in 0..2147483647 range"},
		{[]string{"-w", "5000000000"}, "invalid number '5000000000'"},
		{[]string{"-b", "x"}, "invalid body numbering style: 'x'"},
	} {
		if _, _, err := runPermuted(t, view, input, append([]string{"nl"}, test.args...)...); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("nl %q: %v, want %q", test.args, err, test.want)
		}
	}
}

// Numbers carry on from one FILE to the next, past one that cannot be read, which is named.
func TestNl_carriesNumbersAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "one"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "two"), []byte("c\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runPermuted(t, view, "", "nl", "one", "missing", "two")
	if stdout != "     1\ta\n     2\tb\n     3\tc\n" || stderr != "nl: missing: No such file or directory\n" || err == nil {
		t.Errorf("nl one missing two: %q, %q, %v; want the numbers to carry on and missing named", stdout, stderr, err)
	}
}

// The filters that read any number of files name one they cannot open as busybox's
// fopen_or_warn names it, alone or among others, and read the rest. They said
// `cannot open 'missing'`, which is how an applet that gives up at one words it.
func TestTextFilters_nameAFileTheyCannotOpenAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	// Long enough for strings, which prints runs of four or more.
	if err := os.WriteFile(filepath.Join(dir, "one"), []byte("abcdef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// od is one stream of its FILEs, and says the same; see TestDump_isOneSqueezedStreamAsBusyboxHasIt.
	for _, applet := range []string{"tac", "rev", "nl", "expand", "unexpand", "fold", "strings"} {
		if _, _, err := runPermuted(t, view, "", applet, "missing"); err == nil || err.Error() != "missing: No such file or directory" {
			t.Errorf("%s missing: %v, want busybox's missing: No such file or directory", applet, err)
		}
		stdout, stderr, err := runPermuted(t, view, "", applet, "missing", "one")
		if stderr != applet+": missing: No such file or directory\n" || stdout == "" || err == nil {
			t.Errorf("%s missing one: %q, %q, %v; want missing named and one read", applet, stdout, stderr, err)
		}
	}
}
