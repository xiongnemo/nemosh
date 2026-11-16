package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A move across volumes keeps what a rename on one volume keeps: the bytes, the alternate data
// streams, the hidden bit and the time, of a small file as of a large one and of the files in a
// directory, and the source is gone after. The rename is made to say the volumes differ, so the
// move that follows is the one a move to another drive makes.
func TestMv_acrossVolumesKeepsWhatARenameKeeps(t *testing.T) {
	saved := moveRename
	t.Cleanup(func() { moveRename = saved })
	moveRename = func(source, dest string) error {
		return &os.LinkError{Op: "rename", Old: source, New: dest, Err: errorNotSameDevice}
	}
	dir := t.TempDir()
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	made := func(path string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("the bytes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+":side", []byte("a stream"), 0o644); err != nil {
			t.Skipf("no alternate data streams here: %v", err)
		}
		name, _ := windows.UTF16PtrFromString(path)
		if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_ARCHIVE); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatal(err)
		}
	}
	made(filepath.Join(dir, "file"))
	if err := os.Mkdir(filepath.Join(dir, "tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	made(filepath.Join(dir, "tree", "inner"))

	applet, _ := DefaultRegistry.Lookup("mv")
	for _, test := range []struct{ from, to, check string }{
		{from: "file", to: "moved", check: "moved"},
		{from: "tree", to: "elsewhere", check: filepath.Join("elsewhere", "inner")},
	} {
		var stderr bytes.Buffer
		ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: os.Environ()})
		if err := applet.Run(ctx, []string{test.from, test.to}, strings.NewReader(""), &bytes.Buffer{}, &stderr); err != nil {
			t.Fatalf("mv %s %s: %v, %q", test.from, test.to, err, stderr.String())
		}
		if _, err := os.Lstat(filepath.Join(dir, test.from)); !os.IsNotExist(err) {
			t.Errorf("%s is still there: %v", test.from, err)
		}
		moved := filepath.Join(dir, test.check)
		if data, err := os.ReadFile(moved); err != nil || string(data) != "the bytes\n" {
			t.Errorf("%s holds %q, %v", test.check, data, err)
		}
		if data, err := os.ReadFile(moved + ":side"); err != nil || string(data) != "a stream" {
			t.Errorf("%s lost its alternate data stream: %q, %v", test.check, data, err)
		}
		name, _ := windows.UTF16PtrFromString(moved)
		if attributes, err := windows.GetFileAttributes(name); err != nil || attributes&windows.FILE_ATTRIBUTE_HIDDEN == 0 {
			t.Errorf("%s has attributes %#x, %v; want the hidden bit kept", test.check, attributes, err)
		}
		if info, err := os.Stat(moved); err != nil || !info.ModTime().Equal(past) {
			t.Errorf("%s has time %v, %v; want %v", test.check, info.ModTime(), err, past)
		}
	}
}
