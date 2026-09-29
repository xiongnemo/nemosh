package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// rm refuses an operand whose last component is . or .., as busybox's does. It had no such
// check, so `rm -rf ..` removed everything in the parent directory.
func TestRm_refusesDotAndDotDot(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(work, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{filepath.Join(root, "a", "keep"), filepath.Join(work, "mine")} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	view := permuteTestView{cwd: work}
	for _, args := range [][]string{
		{"rm", "-rf", ".."}, {"rm", "-rf", "../"}, {"rm", "-rf", "."}, {"rm", "-rf", "./."},
		{"rm", "-r", "sub/.."}, {"rm", "."},
	} {
		_, stderr, err := runPermuted(t, view, "", args...)
		if want := "rm: cannot remove '.' or '..'\n"; stderr != want || err == nil {
			t.Errorf("%q: got %q, %v; want %q and status 1", args, stderr, err, want)
		}
	}
	for _, name := range []string{filepath.Join(root, "a", "keep"), filepath.Join(work, "mine"), filepath.Join(work, "sub")} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s is gone: %v", name, err)
		}
	}
}

// rm takes -R, -i and -v, as busybox's does, and of -f and -i the later wins. All three were
// refused as invalid options.
func TestRm_takesBusyboxsOptions(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for _, name := range []string{"g", "tree"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"g/h", "tree/leaf", "i", "readonly"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(dir, "readonly"), 0o444); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args               []string
		stdin              string
		wantStdout, wantEr string
		gone               string
	}{
		{[]string{"rm", "-rv", "g"}, "", "removed 'g/h'\nremoved directory: 'g'\n", "", "g"},
		{[]string{"rm", "-R", "tree"}, "", "", "", "tree"},
		{[]string{"rm", "-i", "i"}, "n\n", "", "rm: remove 'i'? ", ""},
		{[]string{"rm", "-iv", "i"}, "y\n", "removed 'i'\n", "rm: remove 'i'? ", "i"},
		{[]string{"rm", "-if", "nosuch"}, "", "", "", ""},
		{[]string{"rm", "readonly"}, "", "", "", "readonly"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.stdin, test.args...)
		if stdout != test.wantStdout || stderr != test.wantEr || err != nil {
			t.Errorf("%q: got %q, %q, %v; want %q, %q", test.args, stdout, stderr, err, test.wantStdout, test.wantEr)
		}
		if test.gone != "" {
			if _, err := os.Lstat(filepath.Join(dir, test.gone)); !os.IsNotExist(err) {
				t.Errorf("%q left %s there: %v", test.args, test.gone, err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "i")); !os.IsNotExist(err) {
		t.Error("rm -iv i answered y and left i")
	}
	_, stderr, err := runPermuted(t, view, "", "rm", "-fi", "nosuch")
	if want := "rm: cannot remove 'nosuch': No such file or directory\n"; stderr != want || err == nil {
		t.Errorf("rm -fi nosuch: got %q, %v; want %q, since -i came last", stderr, err, want)
	}
}
