package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tar -c compresses with -z's gzip, -j's bzip2, -J's xz and --lzma's lzma, and under -a with
// what the name ends in, a .tgz as a .tar.gz, as busybox's tar reads the name; and each comes
// back out under the same letter. -j wrote a plain tar under the name and exited 0, and -a
// wrote a .tgz plain; then all but gzip were refused, until each had a writer. busybox's tar
// writes xz and lzma through a program it finds on PATH, which a Windows machine has not got.
func TestTar_createCompressesWithEachCodec(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "hi\n"})
	magic := map[string]string{"gzip": "\x1f\x8b", "bzip2": "BZh", "xz": "\xfd7zXZ\x00", "lzma": "\x5d", "": "a.txt"}
	for _, test := range []struct {
		args   []string
		method string
		list   []string
	}{
		{args: []string{"-caf", "t.tgz", "a.txt"}, method: "gzip", list: []string{"-tzf"}},
		{args: []string{"-caf", "t.tar.gz", "a.txt"}, method: "gzip", list: []string{"-taf"}},
		{args: []string{"-czf", "z.tar", "a.txt"}, method: "gzip", list: []string{"-tzf"}},
		{args: []string{"-cjf", "t.tbz", "a.txt"}, method: "bzip2", list: []string{"-tjf"}},
		{args: []string{"-caf", "t.tar.bz2", "a.txt"}, method: "bzip2", list: []string{"-taf"}},
		{args: []string{"-cJf", "t.txz", "a.txt"}, method: "xz", list: []string{"-tJf"}},
		{args: []string{"-caf", "t.tar.xz", "a.txt"}, method: "xz", list: []string{"-taf"}},
		{args: []string{"--lzma", "-cf", "t.tlz", "a.txt"}, method: "lzma", list: []string{"--lzma", "-tf"}},
		{args: []string{"-caf", "t.tar.lzma", "a.txt"}, method: "lzma", list: []string{"-taf"}},
		{args: []string{"-caf", "a.tar", "a.txt"}, method: "", list: []string{"-tf"}},
		{args: []string{"-cf", "p.tar", "a.txt"}, method: "", list: []string{"-tf"}},
	} {
		name := test.args[len(test.args)-2]
		if _, stderr, err := runSmall(t, dir, "", "tar", test.args...); err != nil {
			t.Errorf("tar %q: %q, %v", test.args, stderr, err)
			continue
		}
		if data, _ := os.ReadFile(filepath.Join(dir, name)); !strings.HasPrefix(string(data), magic[test.method]) {
			t.Errorf("tar %q wrote no %s: %q", test.args, test.method, data[:min(len(data), 6)])
		}
		if listed, stderr, err := runSmall(t, dir, "", "tar", append(test.list, name)...); listed != "a.txt\n" || err != nil {
			t.Errorf("tar %q %s = %q, %q, %v", test.list, name, listed, stderr, err)
		}
	}
}
