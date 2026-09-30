package applets_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shred overwrites each FILE, with zeros last under -z, to -s's size, removes it with -u, and
// makes a read-only one writable with -f, as busybox's does. A FILE it cannot open ends it there.
// There was no shred.
func TestShred_overwritesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	write := func(name, text string, perm os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), perm); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	write("f", "secret text\n", 0o644)
	if _, stderr, err := runPermuted(t, view, "", "shred", "f"); err != nil || stderr != "" {
		t.Fatalf("shred f: %q, %v", stderr, err)
	}
	if got := read("f"); len(got) != 12 || bytes.Equal(got, []byte("secret text\n")) {
		t.Errorf("shred f left %q; want 12 other bytes", got)
	}
	write("z", "secret\n", 0o644)
	if _, _, err := runPermuted(t, view, "", "shred", "-n", "1", "-z", "z"); err != nil {
		t.Fatal(err)
	}
	if got := read("z"); !bytes.Equal(got, make([]byte, 7)) {
		t.Errorf("shred -z left %q; want seven zeros", got)
	}
	write("s", "x\n", 0o644)
	if _, _, err := runPermuted(t, view, "", "shred", "-n", "0", "-z", "-s", "0x10", "s"); err != nil {
		t.Fatal(err)
	}
	if got := read("s"); !bytes.Equal(got, make([]byte, 16)) {
		t.Errorf("shred -s 0x10 left %q; want sixteen zeros", got)
	}
	write("u", "gone\n", 0o644)
	write("e", "", 0o644)
	if _, _, err := runPermuted(t, view, "", "shred", "-u", "u", "e"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"u", "e"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("shred -u left %s", name)
		}
	}
	write("ro", "held\n", 0o444)
	if _, _, err := runPermuted(t, view, "", "shred", "ro"); err == nil || !strings.Contains(err.Error(), "cannot open 'ro'") {
		t.Errorf("shred of a read-only file: %v; want it refused", err)
	}
	if _, _, err := runPermuted(t, view, "", "shred", "-f", "-n", "0", "-z", "ro"); err != nil {
		t.Errorf("shred -f of a read-only file: %v", err)
	}
	write("after", "kept\n", 0o644)
	if _, _, err := runPermuted(t, view, "", "shred", "-u", "nosuch", "after"); err == nil || !strings.Contains(err.Error(), "cannot open 'nosuch'") {
		t.Errorf("shred nosuch: %v; want it refused", err)
	}
	if got := read("after"); string(got) != "kept\n" {
		t.Errorf("a FILE after one shred could not open was shredded: %q", got)
	}
	for _, args := range [][]string{{"shred"}, {"shred", "-s", "10k", "f"}, {"shred", "-n", "x", "f"}} {
		if _, _, err := runPermuted(t, view, "", args...); err == nil {
			t.Errorf("%q: accepted; want it refused", args)
		}
	}
}
