package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ls -l's mode column is the one stat makes up, so the two say the same of every entry: on
// Windows busybox-w32's, read and write for everyone less the umask's group and other write, and
// run for a directory or a program. ls printed Go's, `-rw-rw-rw-` beside stat's `-rw-r--r--`.
func TestLs_modeColumnIsStats(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"plain.txt": "x\n", "locked.txt": "y\n", "run.bat": "@echo hi\n"})
	if err := os.Chmod(filepath.Join(dir, "locked.txt"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "locked.txt"), 0o644) })
	for _, name := range []string{"plain.txt", "locked.txt", "run.bat", "sub"} {
		listed, _, err := runSmall(t, dir, "", "ls", "-ld", name)
		if err != nil {
			t.Fatalf("ls -ld %s: %v", name, err)
		}
		stated, _, err := runSmall(t, dir, "", "stat", "-c", "%A", name)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if mode := strings.Fields(listed)[0]; mode != strings.TrimSpace(stated) {
			t.Errorf("ls -ld %s says %s, stat %s", name, mode, strings.TrimSpace(stated))
		}
	}
}
