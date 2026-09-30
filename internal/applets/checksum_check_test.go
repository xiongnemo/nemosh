package applets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// md5sum -c and the rest of the family check a list as busybox's md5_sha1_sum does: every line
// counts, one without a blank fails, a name that cannot be opened is said on stderr and FAILED,
// hashes compare in either case, each list ends with its own `WARNING: N of M computed checksums
// did NOT match`, an empty one is `FILE: no checksum lines found`, and -s prints only what
// could not be opened. -b marks a name with `*`. Each answer is busybox-w32's, measured.
func TestChecksum_checkIsBusyboxs(t *testing.T) {
	const abc, x = "900150983cd24fb0d6963f7d28e17f72", "9dd4e461268c8034f5c8564e155c67a6"
	dir := t.TempDir()
	for name, text := range map[string]string{
		"f": "abc", "g": "x", "empty": "",
		"ok":    abc + "  f\n" + x + "  g\n",
		"blank": abc + "  f\n" + x + "  g\n\n",
		"upper": "900150983CD24FB0D6963F7D28E17F72  f\n",
		"crlf":  abc + "  f\r\n" + x + " *g\r\n",
		"one":   abc + " f\n",
		"bad":   "00000000000000000000000000000000  f\n",
		"gone":  "00000000000000000000000000000000  nosuch\n",
		"sha1":  "a9993e364706816aba3e25717850c26c9cd0d89d  f\n" + abc + "  f\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const cannot = "md5sum: cannot open 'nosuch': No such file or directory\n"
	for _, test := range []struct {
		applet         string
		args           []string
		stdout, stderr string
		status         int
	}{
		{"md5sum", []string{"-c", "ok"}, "f: OK\ng: OK\n", "", 0},
		{"md5sum", []string{"-c", "blank"}, "f: OK\ng: OK\n", "md5sum: WARNING: 1 of 3 computed checksums did NOT match\n", 1},
		{"md5sum", []string{"-cw", "blank"}, "f: OK\ng: OK\n", "md5sum: invalid format\nmd5sum: WARNING: 1 of 3 computed checksums did NOT match\n", 1},
		{"md5sum", []string{"-c", "upper"}, "f: OK\n", "", 0},
		{"md5sum", []string{"-c", "crlf"}, "f: OK\ng: OK\n", "", 0},
		{"md5sum", []string{"-c", "one"}, "f: OK\n", "", 0},
		{"md5sum", []string{"-c", "gone"}, "nosuch: FAILED\n", cannot + "md5sum: WARNING: 1 of 1 computed checksums did NOT match\n", 1},
		{"md5sum", []string{"-c", "bad", "ok"}, "f: FAILED\nf: OK\ng: OK\n", "md5sum: WARNING: 1 of 1 computed checksums did NOT match\n", 1},
		{"md5sum", []string{"-c", "empty"}, "", "md5sum: empty: no checksum lines found\n", 1},
		{"md5sum", []string{"-c", "-s", "bad"}, "", "", 1},
		{"md5sum", []string{"-cs", "ok"}, "", "", 0},
		{"md5sum", []string{"-cs", "gone"}, "", cannot, 1},
		{"sha1sum", []string{"-c", "sha1"}, "f: OK\nf: FAILED\n", "sha1sum: WARNING: 1 of 2 computed checksums did NOT match\n", 1},
		{"md5sum", []string{"-b", "f"}, abc + " *f\n", "", 0},
		{"md5sum", []string{"-bt", "f"}, abc + "  f\n", "", 0},
		{"md5sum", []string{"-tb", "f", "g"}, abc + " *f\n" + x + " *g\n", "", 0},
	} {
		stdout, stderr, err := runChecksum(t, dir, test.applet, "", test.args...)
		status, _ := applets.StatusCode(err)
		if stdout != test.stdout || stderr != test.stderr || status != test.status {
			t.Errorf("%s %q = %q, %q, status %d (%v); want %q, %q, status %d",
				test.applet, test.args, stdout, stderr, status, err, test.stdout, test.stderr, test.status)
		}
	}
	if stdout, _, err := runChecksum(t, dir, "md5sum", "abc", "-b"); err != nil || stdout != abc+" *-\n" {
		t.Errorf("md5sum -b of stdin = %q, %v; want %q", stdout, err, abc+" *-\n")
	}
	for _, letter := range []string{"s", "w"} {
		if _, _, err := runChecksum(t, dir, "md5sum", "", "-"+letter, "f"); err == nil || err.Error() != "-"+letter+" requires -c" {
			t.Errorf("md5sum -%s f: %v, want %q", letter, err, "-"+letter+" requires -c")
		}
	}
}
