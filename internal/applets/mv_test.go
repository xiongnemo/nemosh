package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// mv takes busybox's options: -v, -n, -i, -t, -T and the long forms. It took -f alone, so each of
// these was refused as an invalid option.
func TestMv_takesBusyboxsOptions(t *testing.T) {
	dir, view := cpFixture(t)
	for _, name := range []string{"x", "t1", "t2"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args       []string
		stdin      string
		wantStdout string
		wantStderr string
	}{
		{[]string{"mv", "-v", "f", "moved"}, "", "'f' -> 'moved'\n", ""},
		{[]string{"mv", "-n", "x", "g"}, "", "", ""},
		{[]string{"mv", "-i", "x", "g"}, "n\n", "", "mv: overwrite 'g'? "},
		{[]string{"mv", "-t", "d", "t1", "t2"}, "", "", ""},
		{[]string{"mv", "--verbose", "--target-directory=d", "moved"}, "", "'moved' -> 'd/moved'\n", ""},
	} {
		stdout, stderr, err := runPermuted(t, view, test.stdin, test.args...)
		if stdout != test.wantStdout || stderr != test.wantStderr || err != nil {
			t.Errorf("%q: got %q, %q, %v; want %q, %q", test.args, stdout, stderr, err, test.wantStdout, test.wantStderr)
		}
	}
	if got := readFixture(t, filepath.Join(dir, "g")); got != "old\n" {
		t.Errorf("mv -n and a refused mv -i replaced g with %q", got)
	}
	for _, path := range []string{"d/t1", "d/t2", "d/moved"} {
		readFixture(t, filepath.Join(dir, path))
	}
	if _, _, err := runPermuted(t, view, "y\n", "mv", "-i", "x", "g"); err != nil || readFixture(t, filepath.Join(dir, "g")) != "x\n" {
		t.Errorf("mv -i answered y: %v; want g replaced", err)
	}
}

// A read-only destination is replaced, as busybox-w32's rename replaces one, and what cannot be
// replaced is named.
func TestMv_replacesAndRefusesAsBusyboxDoes(t *testing.T) {
	dir, view := cpFixture(t)
	target := filepath.Join(dir, "g")
	if err := os.Chmod(target, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })
	if _, stderr, err := runPermuted(t, view, "", "mv", "f", "g"); err != nil || stderr != "" {
		t.Fatalf("mv f g over a read-only g: %q, %v", stderr, err)
	}
	if got := readFixture(t, target); got != "hello\n" {
		t.Errorf("g holds %q, want what f held", got)
	}
	if err := os.Mkdir(filepath.Join(dir, "d", "g"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args       []string
		wantStderr string
	}{
		{[]string{"mv", "-T", "g", "d"}, "mv: 'd' is a directory\n"},
		{[]string{"mv", "g", "d"}, "mv: cannot rename 'g': Is a directory\n"},
		{[]string{"mv", "nosuch", "z"}, "mv: cannot rename 'nosuch': No such file or directory\n"},
	} {
		_, stderr, err := runPermuted(t, view, "", test.args...)
		if stderr == "" && err != nil {
			stderr = "mv: " + err.Error() + "\n"
		}
		if stderr != test.wantStderr || err == nil {
			t.Errorf("%q: got %q, %v; want %q and status 1", test.args, stderr, err, test.wantStderr)
		}
	}
}
