package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unxz, xzcat, unlzma and lzcat, and xz and lzma with -d, which read and only read, as
// busybox's do. The fixtures are XZ Utils 5.8.3's, `xz -c` and `xz --format=lzma -c`, so the
// readers are tried on what another program wrote.
var xzFixture = []byte{
	0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00, 0x00, 0x04, 0xe6, 0xd6, 0xb4, 0x46,
	0x04, 0xc0, 0x16, 0x12, 0x21, 0x01, 0x16, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0xa7, 0x51, 0x77, 0x9e, 0x01, 0x00, 0x11, 0x78,
	0x7a, 0x20, 0x74, 0x68, 0x72, 0x6f, 0x75, 0x67, 0x68, 0x20, 0x6e, 0x65,
	0x6d, 0x6f, 0x73, 0x68, 0x0a, 0x00, 0x00, 0x00, 0xd7, 0xcc, 0x45, 0x09,
	0x0c, 0x77, 0x4a, 0x07, 0x00, 0x01, 0x32, 0x12, 0x12, 0x90, 0x4f, 0x3e,
	0x1f, 0xb6, 0xf3, 0x7d, 0x01, 0x00, 0x00, 0x00, 0x00, 0x04, 0x59, 0x5a,
}

const xzPlain = "xz through nemosh\n"

var lzmaFixture = []byte{
	0x5d, 0x00, 0x00, 0x80, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0x00, 0x36, 0x1e, 0x89, 0xdd, 0x7d, 0x49, 0x77, 0x51, 0x43, 0xb3,
	0x16, 0x9d, 0xd5, 0x20, 0x8c, 0xdc, 0x89, 0xb9, 0x02, 0x08, 0xf3, 0x75,
	0x0d, 0x42, 0xff, 0xff, 0xce, 0x2f, 0x00, 0x00,
}

const lzmaPlain = "lzma through nemosh\n"

// Each name reads its format: to stdout, in place of the FILE, and through zcat, whose first
// bytes choose xz as they choose gzip and bzip2. They were not here; zcat refused xz.
func TestXz_readsWhatXzUtilsWrote(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string][]byte{"a.txt.xz": xzFixture, "b.txt.xz": xzFixture, "c.lzma": lzmaFixture, "d.lzma": lzmaFixture} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		applet, want string
		args         []string
	}{
		{applet: "xzcat", args: []string{"a.txt.xz"}, want: xzPlain},
		{applet: "xz", args: []string{"-dc", "a.txt.xz"}, want: xzPlain},
		{applet: "unxz", args: []string{"-c", "a.txt.xz"}, want: xzPlain},
		{applet: "zcat", args: []string{"a.txt.xz"}, want: xzPlain},
		{applet: "lzcat", args: []string{"c.lzma"}, want: lzmaPlain},
		{applet: "lzma", args: []string{"-dc", "c.lzma"}, want: lzmaPlain},
		{applet: "unxz", args: []string{"b.txt.xz"}},
		{applet: "unlzma", args: []string{"d.lzma"}},
	} {
		if got, stderr, err := runSmall(t, dir, "", test.applet, test.args...); got != test.want || err != nil {
			t.Errorf("%s %q = %q, %q, %v; want %q", test.applet, test.args, got, stderr, err, test.want)
		}
	}
	for name, want := range map[string]string{"b.txt": xzPlain, "d": lzmaPlain} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); string(got) != want || err != nil {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
	if got, _, err := runSmall(t, dir, string(xzFixture), "xzcat"); got != xzPlain || err != nil {
		t.Errorf("xzcat from a pipe = %q, %v", got, err)
	}
}

// What they cannot read is said in busybox's words: an xz stream cut short anywhere, even in
// its first block's header, is corrupted data, and an lzma header that will not read is a bad
// lzma header. Data that is not xz at all is invalid magic, where busybox's xzcat writes
// nothing and exits 0. And xz and lzma do not compress, as busybox's do not.
func TestXz_saysWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		applet, stdin, says string
	}{
		{applet: "xzcat", stdin: string(xzFixture[:20]), says: "corrupted data"},
		{applet: "xzcat", stdin: string(xzFixture[:len(xzFixture)-1]), says: "corrupted data"},
		{applet: "xzcat", stdin: "not xz at all", says: "invalid magic"},
		{applet: "lzcat", stdin: string(lzmaFixture[:12]), says: "bad lzma header"},
		{applet: "lzcat", stdin: string(lzmaFixture[:30]), says: "corrupted data"},
		{applet: "xz", stdin: "plain", says: "only -d is here"},
		{applet: "lzma", stdin: "plain", says: "only -d is here"},
	} {
		_, stderr, err := runSmall(t, dir, test.stdin, test.applet)
		if err == nil || !strings.Contains(stderr+err.Error(), test.says) {
			t.Errorf("%s <<< %d bytes = %q, %v; want %q", test.applet, len(test.stdin), stderr, err, test.says)
		}
	}
}
