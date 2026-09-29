package applets_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// xxdIn runs xxd in dir with stdin, and gives its output, what it would print as an error, and
// its status.
func xxdIn(t *testing.T, dir string, stdout io.Writer, stdin string, args ...string) (string, int) {
	t.Helper()
	applet, _ := applets.DefaultRegistry.Lookup("xxd")
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	var stderr bytes.Buffer
	err := applet.Run(ctx, args, strings.NewReader(stdin), stdout, &stderr)
	if err == nil {
		return stderr.String(), 0
	}
	if code, ok := applets.StatusCode(err); ok {
		return stderr.String(), code
	}
	return stderr.String() + err.Error(), 1
}

func xxdDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"h": "hello\n", "q": "The quick brown fox jumps over the lazy dog.", "1st": "ab"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Each layout is busybox's formats for xxd's options over libbb's dump, and each was measured
// against busybox-w32's xxd. xxd had its one layout and -p's, and -p put every byte on one line.
func TestXxd_dumpsAsBusybox(t *testing.T) {
	dir := xxdDir(t)
	for _, test := range []struct {
		args  []string
		stdin string
		want  string
	}{
		{[]string{"h"}, "", "00000000: 6865 6c6c 6f0a                           hello.\n"},
		{[]string{"-p", "q"}, "", "54686520717569636b2062726f776e20666f78206a756d7073206f766572\n20746865206c617a7920646f672e\n"},
		{[]string{"-ps", "h"}, "", "68656c6c6f0a\n"},
		{[]string{"-pc50", "h"}, "", "68656c6c6f0a\n"},
		{[]string{"-p", "-c", "4", "h"}, "", "68656c6c\n6f0a\n"},
		{[]string{"-g", "1", "h"}, "", "00000000: 68 65 6c 6c 6f 0a                                hello.\n"},
		{[]string{"-g", "3", "-l", "16", "q"}, "", "00000000: 546865 207175 69636b 206272 6f776e 20  The quick brown \n"},
		{[]string{"-g", "0", "h"}, "", "00000000: 68656c6c6f0a                      hello.\n"},
		{[]string{"-c", "4", "h"}, "", "00000000: 6865 6c6c  hell\n00000004: 6f0a       o.\n"},
		{[]string{"-s", "2", "h"}, "", "00000002: 6c6c 6f0a                                llo.\n"},
		{[]string{"-o", "-4", "h"}, "", "fffffffffffffffc: 6865 6c6c 6f0a                           hello.\n"},
		{[]string{"-i"}, "hello\n", "  0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x0a,\n"},
		{[]string{"-i", "1st"}, "", "unsigned char __1st[] = {\n  0x61, 0x62,\n};\nunsigned int __1st_len = 2;\n"},
		// The length is where the dump ended, so it counts what -s skipped.
		{[]string{"-i", "-s", "40", "q"}, "", "unsigned char q[] = {\n  0x64, 0x6f, 0x67, 0x2e,\n};\nunsigned int q_len = 44;\n"},
		{[]string{}, "", ""},
	} {
		var stdout bytes.Buffer
		if stderr, status := xxdIn(t, dir, &stdout, test.stdin, test.args...); stdout.String() != test.want || status != 0 {
			t.Errorf("xxd %q: %q, %q, status %d; want %q", test.args, stdout.String(), stderr, status, test.want)
		}
	}
}

// xxd -r is busybox's reverse: each line's bytes put at its address, one stray character
// allowed before a byte and a second ending the line, a byte's digits joined across a line with
// -p. Where stdout is no file, a gap is zeros and an address behind is refused, as busybox's
// fseeko has it; busybox-w32's seek on a pipe succeeds without moving, and loses both.
func TestXxd_reversesAsBusybox(t *testing.T) {
	dir := xxdDir(t)
	var dump bytes.Buffer
	xxdIn(t, dir, &dump, "", "q")
	for _, test := range []struct {
		args              []string
		stdin, want, fail string
	}{
		{[]string{"-r"}, dump.String(), "The quick brown fox jumps over the lazy dog.", ""},
		{[]string{"-r", "-p"}, "54686520717569\n636b\n", "The quick", ""},
		{[]string{"-r", "-p"}, "3\n1 0a\n", "1\n", ""},
		{[]string{"-r", "-p"}, "31 !3 0a 0a\n", "10\xa0", ""},
		{[]string{"-r", "-p"}, "31 !!343434\n30 0a\n", "10\n", ""},
		{[]string{"-r"}, "00000000: 41 42  AB\n", "AB", ""},
		{[]string{"-r"}, "  00000000 41 42\n", "AB", ""},
		{[]string{"-r"}, "00000000: 414\n2 43\n", "A\x00C", ""},
		{[]string{"-r"}, "00000004: 41\n", "\x00\x00\x00\x00A", ""},
		{[]string{"-r", "-s", "3"}, "00000000: 41\n", "\x00\x00\x00A", ""},
		{[]string{"-r"}, "00000004: 41\n00000000: 42\n", "\x00\x00\x00\x00A", "cannot seek: Illegal seek"},
		{[]string{"-r"}, "00000000: 4142\n\n00000002: 4344\n", "AB", "invalid number ''"},
		{[]string{"-r"}, "zz: 4142\n", "", "invalid number 'zz: 4142'"},
	} {
		var stdout bytes.Buffer
		stderr, status := xxdIn(t, dir, &stdout, test.stdin, test.args...)
		if stdout.String() != test.want || !strings.Contains(stderr, test.fail) || (test.fail != "") != (status != 0) {
			t.Errorf("xxd %q < %q: %q, %q, status %d; want %q and %q", test.args, test.stdin, stdout.String(), stderr, status, test.want, test.fail)
		}
	}
	// Into a file an address seeks, and one behind patches what is there.
	out, err := os.Create(filepath.Join(dir, "patched"))
	if err != nil {
		t.Fatal(err)
	}
	xxdIn(t, dir, out, "00000004: 41\n00000000: 42\n", "-r")
	out.Close()
	if got, _ := os.ReadFile(filepath.Join(dir, "patched")); string(got) != "B\x00\x00\x00A" {
		t.Errorf("xxd -r into a file: %q, want B, three zeros and A", got)
	}
}

// Numbers are read as busybox reads each: -l and -s as C numbers, -g and -c as decimal ones,
// none of them signed. One FILE at most, and one that cannot be opened is named.
func TestXxd_refusesAsBusybox(t *testing.T) {
	dir := xxdDir(t)
	for _, test := range []struct {
		args      []string
		out, fail string
	}{
		{[]string{"-l", "x", "h"}, "", "invalid number 'x'"},
		{[]string{"-c", "x", "h"}, "", "invalid number 'x'"},
		{[]string{"-g", "-1", "h"}, "", "invalid number '-1'"},
		{[]string{"-s", "-1", "h"}, "", "invalid number '-1'"},
		{[]string{"-l", "99999999999", "h"}, "", "invalid number '99999999999'"},
		{[]string{"h", "q"}, "", "extra operand 'q'"},
		{[]string{"nosuch"}, "", "nosuch: No such file or directory"},
		{[]string{"-i", "nosuch"}, "unsigned char nosuch[] = {\n", "nosuch: No such file or directory"},
		{[]string{"-r", "nosuch"}, "", "cannot open 'nosuch': No such file or directory"},
	} {
		var stdout bytes.Buffer
		stderr, status := xxdIn(t, dir, &stdout, "", test.args...)
		if stdout.String() != test.out || !strings.Contains(stderr, test.fail) || status != 1 {
			t.Errorf("xxd %q: %q, %q, status %d; want %q and %q", test.args, stdout.String(), stderr, status, test.out, test.fail)
		}
	}
}
