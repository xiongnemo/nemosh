package applets_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// gzipMember is a gzip of text whose header holds stamp as its MTIME.
func gzipMember(t *testing.T, text string, stamp time.Time) string {
	t.Helper()
	var member bytes.Buffer
	writer := gzip.NewWriter(&member)
	writer.ModTime = stamp
	if _, err := writer.Write([]byte(text)); err != nil || writer.Close() != nil {
		t.Fatal(err)
	}
	return member.String()
}

// gzip and gunzip take a FILE in busybox's order, so each failure is the one busybox's
// reports: a FILE not there, then one that cannot be opened, then a name with no suffix,
// then a companion already there. The name was looked at first, so a FILE not there was an
// unknown suffix. - is standard input, which was looked for as a file. Each answer was
// measured against busybox-w32, but a directory's, which busybox-w32 cannot open with
// `Permission denied`; it is `Is a directory` here, as cat and md5sum say it.
func TestGzip_takesAFileInBusyboxsOrder(t *testing.T) {
	good := gzipMember(t, "hello world\n", time.Time{})
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello world\n", "k.txt": "keep\n", "k.txt.gz": "", "d/x": ""})
	for _, test := range []struct {
		stdin, applet string
		args          []string
		out, err      string
	}{
		{applet: "gunzip", args: []string{"nope"}, err: "gunzip: nope: No such file or directory\n"},
		{applet: "gzip", args: []string{"d"}, err: "gzip: cannot open 'd': Is a directory\n"},
		{applet: "gunzip", args: []string{"d"}, err: "gunzip: cannot open 'd': Is a directory\n"},
		{applet: "gzip", args: []string{"k.txt"}, err: "gzip: cannot open 'k.txt.gz': File exists\n"},
		{stdin: good, applet: "zcat", args: []string{"-"}, out: "hello world\n"},
		{stdin: good, applet: "gunzip", args: []string{"-"}, out: "hello world\n"},
		{stdin: "x", applet: "gunzip", args: []string{"-t", "-"}, err: "gunzip: invalid magic\n"},
	} {
		want := 0
		if test.err != "" {
			want = 1
		}
		out, err, status := compressSays(t, dir, test.stdin, test.applet, test.args...)
		if out != test.out || err != test.err || status != want {
			t.Errorf("%s %q = %q, %q, %d; want %q, %q, %d", test.applet, test.args, out, err, status, test.out, test.err, want)
		}
	}
	if _, err, _ := compressSays(t, dir, "", "gzip", "-f", "k.txt"); err != "" {
		t.Errorf("gzip -f over k.txt.gz: %q", err)
	}
	if _, err, _ := compressSays(t, dir, "", "gzip", "h.txt"); err != "" {
		t.Fatalf("gzip h.txt: %q", err)
	}
	if _, err, _ := compressSays(t, dir, "", "gzip", "h.txt"); err != "gzip: h.txt: No such file or directory\n" {
		t.Errorf("a second gzip h.txt = %q; want h.txt named as not there", err)
	}
}

// gunzip gives the file it writes the time the last member holds, as busybox's sets it, and
// a member that holds none leaves the time it was written. The new file has the original's
// permissions, less the umask's: a 0600 file's archive is 0600, where it was 0644. Each was
// measured against busybox.
func TestGunzip_keepsTheStoredTimeAndTheMode(t *testing.T) {
	stamp := time.Unix(1577934245, 0)
	dir := writeSmallFixture(t, map[string]string{
		"m.txt.gz": gzipMember(t, "one\n", time.Unix(1000000000, 0)) + gzipMember(t, "two\n", stamp),
		"z.txt.gz": gzipMember(t, "zero\n", time.Time{}),
		"p.txt":    "private\n",
	})
	for _, name := range []string{"m.txt.gz", "z.txt.gz"} {
		if _, err, _ := compressSays(t, dir, "", "gunzip", name); err != "" {
			t.Fatalf("gunzip %s: %q", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "m.txt")); err != nil {
		t.Error(err)
	} else if !info.ModTime().Equal(stamp) {
		t.Errorf("gunzip m.txt.gz made m.txt at %v; want %v", info.ModTime(), stamp)
	}
	if info, err := os.Stat(filepath.Join(dir, "z.txt")); err != nil {
		t.Error(err)
	} else if time.Since(info.ModTime()) > time.Hour {
		t.Errorf("gunzip z.txt.gz made z.txt at %v; want the time it was written", info.ModTime())
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(filepath.Join(dir, "p.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err, _ := compressSays(t, dir, "", "gzip", "p.txt"); err != "" {
		t.Fatalf("gzip p.txt: %q", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "p.txt.gz")); err != nil {
		t.Error(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("gzip of a 0600 file made %v; want 0600", info.Mode().Perm())
	}
}
