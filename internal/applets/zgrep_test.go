package applets_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// zgrep, zegrep and zfgrep are grep over what gzip, bzip2 or xz data holds, and over a plain file
// as it is, as gzip's zgrep reads it with `gzip -cdf`: rotated logs are `syslog.2.gz` beside
// `syslog`, and `zgrep error /var/log/syslog*` reads both. There was no zgrep.
func TestZgrep_readsCompressedAndPlainFilesAlike(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plain.txt", []byte("alpha\nbeta\n"))
	write("a.gz", gzipped(t, "gamma\nalpha2\n"))
	bz, _, err := runAppletWithInput(t, "delta\nbeta2\n", "bzip2", "-c")
	if err != nil {
		t.Fatalf("bzip2 -c: %v", err)
	}
	write("b.bz2", []byte(bz))
	write("broken.gz", []byte{0x1f, 0x8b, 0x08, 0, 0, 0, 0, 0, 0, 0xff, 'n', 'o', 't'})

	for _, test := range []struct {
		applet string
		args   []string
		stdin  string
		want   string
		status int
	}{
		{applet: "zgrep", args: []string{"alpha", "plain.txt", "a.gz"}, want: "plain.txt:alpha\na.gz:alpha2\n"},
		{applet: "zgrep", args: []string{"-c", "alpha", "a.gz"}, want: "1\n"},
		{applet: "zgrep", args: []string{"beta", "b.bz2"}, want: "beta2\n"},
		{applet: "zgrep", args: []string{"gamma"}, stdin: string(gzipped(t, "gamma\n")), want: "gamma\n"},
		{applet: "zgrep", args: []string{"gamma", "-"}, stdin: string(gzipped(t, "gamma\n")), want: "gamma\n"},
		{applet: "zegrep", args: []string{"al(pha|x)2", "a.gz"}, want: "alpha2\n"},
		{applet: "zfgrep", args: []string{"a.p", "plain.txt"}, status: 1},
		{applet: "zfgrep", args: []string{"pha2", "a.gz"}, want: "alpha2\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.stdin, test.applet, test.args...)
		status := 0
		switch {
		case errors.Is(err, applets.ErrExitFalse):
			// grep's nothing matched, which the shell reports as 1.
			status = 1
		case err != nil:
			status, _ = applets.StatusCode(err)
		}
		if stdout != test.want || status != test.status {
			t.Errorf("%s %q = %q, status %d (%s %v); want %q, %d", test.applet, test.args, stdout, status, stderr, err, test.want, test.status)
		}
	}

	for _, test := range []struct {
		args []string
		says string
	}{
		{args: []string{"x", "broken.gz"}, says: "broken.gz"},
		{args: []string{"-r", "x", "."}, says: "-r"},
	} {
		_, stderr, err := runAppletWithInput(t, "", "zgrep", test.args...)
		status, _ := applets.StatusCode(err)
		if reported := stderr + errorText(err); status != 2 || !strings.Contains(reported, test.says) {
			t.Errorf("zgrep %q: status %d, %q; want 2 and %q", test.args, status, reported, test.says)
		}
	}
}

func gzipped(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
