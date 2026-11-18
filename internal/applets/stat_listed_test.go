package applets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// What a directory listing already says of an entry spares the stat its GetFileAttributesEx, which
// was a third of `ls -l`'s time over System32, measured by profile: the record made from the
// listing is the record made from the path, field by field, and asks no GetFileAttributesEx of
// its own. The size and times come from the handle the stat opens anyway, so they are as fresh.
func TestHostStatRecord_fromAListingAsFromThePath(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plain.txt", "some text\n")
	write("script", "#!/bin/sh\necho hi\n")
	write("tool.exe", "MZ")
	write("empty", "")
	if err := os.Chmod(filepath.Join(dir, "plain.txt"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub", "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(dir, "junction"), filepath.Join(dir, "sub")).CombinedOutput(); err != nil {
		t.Logf("no junction: %v %s", err, output)
	}
	_ = os.Symlink("plain.txt", filepath.Join(dir, "link"))
	directories := []string{dir, filepath.Join(dir, "sub")}
	if system, err := os.ReadDir(`C:\Windows\System32`); err == nil && len(system) > 0 {
		directories = append(directories, `C:\Windows\System32`)
	}
	saved := lstatOf
	t.Cleanup(func() { lstatOf = saved })
	compared := 0
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) > 400 {
			entries = entries[:400]
		}
		for _, entry := range entries {
			native := filepath.Join(directory, entry.Name())
			listed, err := entry.Info()
			if err != nil {
				continue
			}
			lstatOf = saved
			byPath, pathErr := hostStatRecord(native, false, 0o022)
			calls := 0
			lstatOf = func(name string) (os.FileInfo, error) { calls++; return saved(name) }
			fromListing, listErr := hostStatRecordListed(native, 0o022, listed)
			if (pathErr == nil) != (listErr == nil) {
				t.Errorf("%s: by path %v, from the listing %v", native, pathErr, listErr)
				continue
			}
			// A stat reads a file's first bytes for its execute bits, and a directory's entries for
			// its links, and where the access time cannot be held that reading moves it: the second
			// of two stats in a row sees the first's. The rest must agree exactly.
			fromListing.access = byPath.access
			if byPath != fromListing {
				t.Errorf("%s:\nby path      %+v\nfrom listing %+v", native, byPath, fromListing)
			}
			// Only an entry whose handle cannot be had asks GetFileAttributesEx, for its size and
			// times, as a listing gives a directory none.
			if opened := fromListing.device != 0 || fromListing.inode != 0; opened && calls != 0 || calls > 1 {
				t.Errorf("%s: the stat from a listing asked GetFileAttributesEx %d times", native, calls)
			}
			compared++
		}
	}
	if compared < 10 {
		t.Fatalf("compared %d entries, which proves little", compared)
	}
}
