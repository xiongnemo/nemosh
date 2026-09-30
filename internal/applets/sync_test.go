package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sync flushes and ignores its arguments, saying so, and fsync flushes each FILE, naming one it
// cannot open, as busybox's do. Neither applet was here.
func TestSyncAndFsync_flushAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if _, stderr, err := runPermuted(t, view, "", "sync"); err != nil || stderr != "" {
		t.Errorf("sync: %q, %v; want nothing said", stderr, err)
	}
	if _, stderr, err := runPermuted(t, view, "", "sync", "x", "-Z"); err != nil || stderr != "sync: ignoring all arguments\n" {
		t.Errorf("sync x -Z: %q, %v; want the arguments ignored, as busybox's are", stderr, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file that cannot be written is flushed as far as it can be, and passes.
	if err := os.WriteFile(filepath.Join(dir, "ro"), []byte("data\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runPermuted(t, view, "", "fsync", "f", "ro", "."); err != nil || stderr != "" {
		t.Errorf("fsync f ro .: %q, %v; want each flushed", stderr, err)
	}
	if _, stderr, err := runPermuted(t, view, "", "fsync", "-d", "f"); err != nil || stderr != "" {
		t.Errorf("fsync -d f: %q, %v", stderr, err)
	}
	_, stderr, err := runPermuted(t, view, "", "fsync", "nosuch", "f")
	if err == nil || stderr != "fsync: cannot open 'nosuch': No such file or directory\n" {
		t.Errorf("fsync nosuch f: %q, %v; want nosuch named and the status 1", stderr, err)
	}
	if _, _, err := runPermuted(t, view, "", "fsync"); err == nil || !strings.Contains(err.Error(), "missing operand") {
		t.Errorf("fsync with no FILE: %v; want it refused", err)
	}
}
