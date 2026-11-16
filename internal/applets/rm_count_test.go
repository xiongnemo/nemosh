package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// rm -r stat'd each entry of a directory it had just listed, whose listing had said what the
// stat did, and then stat'd each file again to be sure it was no directory before removing it.
// The operand is still stat'd, so that a link named is removed and not followed.
//
// Not parallel: it counts through a package variable, which only a sequential test may swap.
func TestRm_asksNothingOfWhatItsDirectoryListed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	for _, name := range []string{"a", "b", "c", "sub/d", "sub/e"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stats := 0
	saved := rmLstat
	defer func() { rmLstat = saved }()
	rmLstat = func(name string) (os.FileInfo, error) { stats++; return os.Lstat(name) }

	// When
	_, stderr, status := runApplet(t, "rm", []string{"-r", dir}, "")

	// Then
	if _, err := os.Lstat(dir); status != 0 || !os.IsNotExist(err) {
		t.Fatalf("rm -r: status %d, %q; the tree is still there: %v", status, stderr, err)
	}
	if stats != 1 {
		t.Fatalf("stat'd %d entries, want the operand alone", stats)
	}
}
