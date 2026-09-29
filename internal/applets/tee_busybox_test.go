package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// tee writes what it can, as busybox's does: a FILE that cannot be opened is named and the
// rest, stdout among them, still get every byte, the status then 1. It stopped at such a FILE
// before copying anything, so what came down the pipe went nowhere. - is stdout again.
func TestTee_writesWhatItCanAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	stdout, stderr, err := runPermuted(t, view, "hi\n", "tee", "one", "nodir/x", "two")
	if stdout != "hi\n" || stderr != "tee: nodir/x: No such file or directory\n" || err == nil {
		t.Errorf("tee one nodir/x two: %q, %q, %v; want hi on stdout, nodir/x named, status 1", stdout, stderr, err)
	}
	for _, name := range []string{"one", "two"} {
		if body, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(body) != "hi\n" {
			t.Errorf("%s holds %q, %v; want hi", name, body, err)
		}
	}
	if stdout, _, err := runPermuted(t, view, "hi\n", "tee", "-"); stdout != "hi\nhi\n" || err != nil {
		t.Errorf("tee -: %q, %v; want hi twice", stdout, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "-")); !os.IsNotExist(err) {
		t.Errorf("tee - made a file named -: %v", err)
	}
	if _, _, err := runPermuted(t, view, "more\n", "tee", "-a", "one"); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "one")); string(body) != "hi\nmore\n" {
		t.Errorf("tee -a one: one holds %q, want hi then more", body)
	}
}
