package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tar -c writes gzip for -z, and under -a for a name that ends in gz, a .tgz as a .tar.gz, as
// busybox's tar reads the name. bzip2, xz and lzma it cannot write, and it says so before the
// archive is opened. -j wrote a plain tar under the name and exited 0, and -a wrote a .tgz plain.
func TestTar_createCompressesOnlyWhatItCan(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "hi\n"})
	gzipped := func(name string) bool {
		data, err := os.ReadFile(filepath.Join(dir, name))
		return err == nil && len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
	}
	for _, test := range []struct {
		args []string
		gzip bool
	}{
		{args: []string{"-caf", "t.tgz", "a.txt"}, gzip: true},
		{args: []string{"-caf", "t.tar.gz", "a.txt"}, gzip: true},
		{args: []string{"-czf", "z.tar", "a.txt"}, gzip: true},
		{args: []string{"-caf", "a.tar", "a.txt"}, gzip: false},
		{args: []string{"-cf", "p.tar", "a.txt"}, gzip: false},
	} {
		if _, stderr, err := runSmall(t, dir, "", "tar", test.args...); err != nil {
			t.Errorf("tar %q: %q, %v", test.args, stderr, err)
		} else if got := gzipped(test.args[1]); got != test.gzip {
			t.Errorf("tar %q: gzip %v, want %v", test.args, got, test.gzip)
		}
	}
	for _, args := range [][]string{{"-cjf", "t.tbz", "a.txt"}, {"-caf", "t.tar.bz2", "a.txt"}, {"-caf", "t.txz", "a.txt"}} {
		_, stderr, err := runSmall(t, dir, "", "tar", args...)
		if err == nil || !strings.Contains(stderr+err.Error(), "cannot compress with") {
			t.Errorf("tar %q = %q, %v; want a refusal", args, stderr, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, args[1])); err == nil {
			t.Errorf("tar %q made %s", args, args[1])
		}
	}
}
