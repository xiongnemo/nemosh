package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// cmpIn runs cmp in a directory holding a, b, c and p, and gives its stdout, its stderr and
// the error it ended with, and its status.
func cmpIn(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"a": "abc\ndef\n", "b": "abc\nxyz\n", "c": "abc\ndef\n", "p": "abc\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	applet, _ := applets.DefaultRegistry.Lookup("cmp")
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	var stdout, stderr bytes.Buffer
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	code, ok := applets.StatusCode(err)
	if !ok {
		code = 1
	}
	if message := err.Error(); !strings.HasPrefix(message, "exit status") {
		stderr.WriteString(message)
	}
	return stdout.String(), stderr.String(), code
}

// cmp is busybox's: `byte` where it said `char`, the EOF on stderr, FILE2 stdin when it is not
// given, SKIP1 and SKIP2, -n, and -l listing each difference, where it read -l and printed the
// first. Every case was measured against busybox-w32's cmp.
func TestCmp_comparesAsBusybox(t *testing.T) {
	for _, test := range []struct {
		args                  []string
		stdin, stdout, stderr string
		status                int
	}{
		{[]string{"a", "b"}, "", "a b differ: byte 5, line 2\n", "", 1},
		{[]string{"a", "c"}, "", "", "", 0},
		{[]string{"a", "p"}, "", "", "cmp: EOF on p\n", 1},
		{[]string{"p", "a"}, "", "", "cmp: EOF on p\n", 1},
		{[]string{"-l", "a", "b"}, "", "5 144 170\n6 145 171\n7 146 172\n", "", 1},
		{[]string{"-l", "a", "p"}, "", "", "cmp: EOF on p\n", 1},
		{[]string{"-s", "a", "b"}, "", "", "", 1},
		{[]string{"-s", "a", "nosuch"}, "", "", "", 2},
		{[]string{"-n", "4", "a", "b"}, "", "", "", 0},
		{[]string{"-n", "5", "a", "b"}, "", "a b differ: byte 5, line 2\n", "", 1},
		{[]string{"a", "b", "4", "4"}, "", "a b differ: byte 1, line 1\n", "", 1},
		{[]string{"a", "b", "1k"}, "", "", "cmp: EOF on a\n", 1},
		{[]string{"--bytes=5", "a", "b"}, "", "a b differ: byte 5, line 2\n", "", 1},
		{[]string{"--quiet", "a", "b"}, "", "", "", 1},
		{[]string{"--verbose", "a", "b"}, "", "5 144 170\n6 145 171\n7 146 172\n", "", 1},
		{[]string{"a"}, "abc\nxyz\n", "a - differ: byte 5, line 2\n", "", 1},
		{[]string{"-", "-"}, "x", "", "", 0},
	} {
		stdout, stderr, status := cmpIn(t, test.stdin, test.args...)
		if stdout != test.stdout || stderr != test.stderr || status != test.status {
			t.Errorf("cmp %q: %q, %q, status %d; want %q, %q and %d", test.args, stdout, stderr, status, test.stdout, test.stderr, test.status)
		}
	}
}

// What cmp cannot use it names, a FILE with status 2 and the rest with 1, before comparing.
func TestCmp_refusesAsBusybox(t *testing.T) {
	for _, test := range []struct {
		args   []string
		want   string
		status int
	}{
		{[]string{"a", "nosuch"}, "nosuch: No such file or directory", 2},
		{[]string{"-n", "x", "a", "b"}, "invalid number 'x'", 1},
		{[]string{"-l", "-s", "a", "b"}, "mutually exclusive", 1},
		{[]string{"a", "b", "1", "2", "3"}, "extra operand '3'", 1},
		{nil, "missing operand", 1},
	} {
		stdout, stderr, status := cmpIn(t, "", test.args...)
		if stdout != "" || !strings.Contains(stderr, test.want) || status != test.status {
			t.Errorf("cmp %q: %q, %q, status %d; want %q and %d", test.args, stdout, stderr, status, test.want, test.status)
		}
	}
}
