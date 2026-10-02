package applets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A name that cannot be stored is said and passed over, the rest stored, and the status is 1
// after a closing word, as busybox goes on. The archive ended at the first such name, its end
// never written, so `tar tf` of it listed nothing.
func TestTar_goesOnPastANameItCannotStore(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"d/g.txt": "x\n", "f.txt": "hello\n"})
	_, stderr, err := runSmall(t, root, "", "tar", "cf", "c.tar", "d", "nope", "f.txt")
	if status, _ := applets.StatusCode(err); status != 1 ||
		stderr != "tar: nope: No such file or directory\ntar: error exit delayed from previous errors\n" {
		t.Errorf("tar cf d nope f.txt = %q, %v; want nope said and status 1", stderr, err)
	}
	if stdout, _, err := runSmall(t, root, "", "tar", "tf", "c.tar"); stdout != "d/\nd/g.txt\nf.txt\n" || err != nil {
		t.Errorf("the archive holds %q, %v; want d/, d/g.txt and f.txt", stdout, err)
	}
}

// A file that will not open is passed over before its header is written, so the archive
// stays whole; busybox quotes its name, as it does every open that fails.
func TestTar_passesOverAFileItCannotRead(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"d/a.txt": "a\n", "d/b.txt": "b\n"})
	locked := filepath.Join(root, "d", "a.txt")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	if file, err := os.Open(locked); err == nil {
		file.Close()
		t.Skip("a file with no permissions is still read here")
	}
	_, stderr, err := runSmall(t, root, "", "tar", "cf", "c.tar", "d")
	if status, _ := applets.StatusCode(err); status != 1 ||
		stderr != "tar: cannot open 'd/a.txt': Permission denied\ntar: error exit delayed from previous errors\n" {
		t.Errorf("tar cf d = %q, %v; want d/a.txt said and status 1", stderr, err)
	}
	if stdout, _, err := runSmall(t, root, "", "tar", "tf", "c.tar"); stdout != "d/\nd/b.txt\n" || err != nil {
		t.Errorf("the archive holds %q, %v; want d/ and d/b.txt", stdout, err)
	}
}

// The archive is not stored in itself, as busybox passes it over and says so; storing it failed
// as it grew while it was read, and the archive was left broken.
func TestTar_doesNotStoreTheArchiveInItself(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"d/g.txt": "x\n"})
	if _, stderr, err := runSmall(t, root, "", "tar", "cf", "d/a.tar", "d"); err != nil ||
		stderr != "tar: d/a.tar: the archive itself is not stored\n" {
		t.Errorf("tar cf d/a.tar d = %q, %v; want the archive passed over", stderr, err)
	}
	if stdout, _, err := runSmall(t, root, "", "tar", "tf", "d/a.tar"); stdout != "d/\nd/g.txt\n" || err != nil {
		t.Errorf("the archive holds %q, %v; want d/ and d/g.txt", stdout, err)
	}
}
