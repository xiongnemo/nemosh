//go:build windows

package applets

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A junction's links are counted as a directory's are, its target's subdirectories and two, as
// busybox-w32's do_lstat counts whatever carries the directory attribute (win32/mingw.c:811):
// scoop's `current`, a junction to a version, is 10 in busybox's `ls -l` and `stat`, and was 1.
func TestStat_aJunctionsLinksAreItsTargetsSubdirectoriesAndTwo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"real/a", "real/b", "real/c"} {
		if err := os.MkdirAll(name, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", "jn", "real").CombinedOutput(); err != nil {
		t.Skipf("no junction can be made here: %v %s", err, out)
	}

	// When
	stat, stderr, status := runApplet(t, "stat", []string{"-c", "%h", "jn"}, "")
	listing, _, _ := runApplet(t, "ls", []string{"-l"}, "")

	// Then
	if status != 0 || strings.TrimSpace(stat) != "5" {
		t.Fatalf("stat -c %%h jn = %q, %q, status %d; want 5", stat, stderr, status)
	}
	for _, line := range strings.Split(listing, "\n") {
		if strings.HasSuffix(line, " jn -> "+filepath.ToSlash(filepath.Join(dir, "real"))) && strings.Fields(line)[1] != "5" {
			t.Fatalf("ls -l says %q, want 5 links", line)
		}
	}
}

// `ls -l` asked each file twice what it is -- its first bytes read for the execute bit by the
// stat that gave the links, the owner and the inode, and read again for the mode column -- and
// opened it a third time for its owner's name, which that stat had read through its handle.
// Over 4,710 entries in System32 it took 444 ms where busybox-w32 takes 363.
//
// Not parallel: it counts through package variables, which only a sequential test may swap.
func TestLs_longListingAsksEachFileOnce(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "notes.txt", "run.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("text"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	heads, owners := 0, 0
	savedHead, savedOwner := readFileHead, ownerNameOf
	defer func() { readFileHead, ownerNameOf = savedHead, savedOwner }()
	readFileHead = func(native string) ([]byte, error) { heads++; return readHead(native) }
	ownerNameOf = func(path string) string { owners++; return fileOwnerName(path) }

	// When
	stdout, stderr, status := runApplet(t, "ls", []string{"-l", dir}, "")

	// Then: the three without a program's suffix read once each, the .exe not at all
	if status != 0 || strings.Count(stdout, "\n") != 6 {
		t.Fatalf("ls -l: status %d, %q, %q", status, stdout, stderr)
	}
	if heads != 3 || owners != 0 {
		t.Fatalf("read %d files' first bytes, want 3; opened %d for the owner's name, want 0", heads, owners)
	}
}
