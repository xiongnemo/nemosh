//go:build windows

package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// du stat'd every entry and opened every one for its index and its links, which busybox-w32's
// lstat does too: over Go's source tree, 12,031 files, it took 811 ms where `find -size` took 117.
// What it asks of a plain file -- its size, whether it is compressed or sparse, and which file it
// is -- the directory's listing already says, and a plain file is never on another volume.
// Directories are still asked, for the size busybox's lstat reports for one and to be counted
// once.
//
// Not parallel: it counts through package variables, which only a sequential test may swap.
func TestDu_asksAPlainFileNothingItsDirectorysListingSays(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c", "sub/d", "sub/e"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 5000)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stats, identities := 0, 0
	savedStat, savedIdentity := duLstat, duIdentity
	defer func() { duLstat, duIdentity = savedStat, savedIdentity }()
	duLstat = func(name string) (os.FileInfo, error) { stats++; return os.Lstat(name) }
	duIdentity = func(path string, info os.FileInfo) (fileID, int, bool) { identities++; return fileIdentity(path, info) }

	// When
	stdout, stderr, status := runApplet(t, "du", []string{"-a", dir}, "")

	// Then: a line for each of the five files and the two directories, and the two asked
	if status != 0 || strings.Count(stdout, "\n") != 7 {
		t.Fatalf("du -a: status %d, %q, %q", status, stdout, stderr)
	}
	if stats != 2 || identities != 2 {
		t.Fatalf("stat'd %d entries and opened %d, want the two directories each", stats, identities)
	}
}
