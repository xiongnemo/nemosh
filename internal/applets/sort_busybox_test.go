package applets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// sort answers as busybox-w32's does (coreutils/sort.c), each case measured there: every
// ordering, keys with their own letters and character offsets, the whole-line tie break and -s,
// -u by key, -c, -z, and the diagnostics, all with status 2 but -c's 1. It took -n -r -u -f -b
// and one -k of whole fields, tied lines came out in no particular order, and -n read integers.
func TestSort_answersAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{[]string{}, "B\na\n_\nb\nA\n", "A\nB\n_\na\nb\n"},
		{[]string{"-V"}, "1.10\n1.9\n1.2\nv1.10a\nv1.9b\n1.02\n1.002\nfoo\n", "1.002\n1.02\n1.2\n1.9\n1.10\nfoo\nv1.9b\nv1.10a\n"},
		// msvcrt's atof reads exponents and no hexadecimal, inf or nan.
		{[]string{"-n"}, "1e3\n0x10\n 2\n-1.5\ninf\nx\n-\n3\n1.25\n1.5\n", "-1.5\n-\n0x10\ninf\nx\n1.25\n1.5\n 2\n3\n1e3\n"},
		{[]string{"-g"}, "1e3\n0x10\n 2\n-1.5\ninf\nx\nnan\n-inf\n3\n0x1p4\n", "x\nnan\n-inf\n-1.5\n 2\n3\n0x10\n0x1p4\n1e3\ninf\n"},
		{[]string{"-h"}, "1K\n2M\n3\n500\n1k\n2G\n1m\n1.5K\n", "1m\n3\n500\n1K\n1k\n1.5K\n2M\n2G\n"},
		{[]string{"-M"}, "Feb\njan\nMAR x\nfoo\n December\nJanuary\n", "foo\nJanuary\njan\nFeb\nMAR x\n December\n"},
		{[]string{"-k2"}, "b 1\na 1\nc 0\n", "c 0\na 1\nb 1\n"},
		{[]string{"-s", "-k2"}, "b 1\na 1\nc 0\n", "c 0\nb 1\na 1\n"},
		{[]string{"-k2", "-r"}, "b 1\na 1\nc 0\n", "b 1\na 1\nc 0\n"},
		{[]string{"-s", "-r", "-k2"}, "b 1\na 1\nc 0\n", "b 1\na 1\nc 0\n"},
		{[]string{"-k2,2r"}, "b 1\na 1\nc 0\n", "a 1\nb 1\nc 0\n"},
		{[]string{"-k2n"}, "x 10\ny 9\n", "y 9\nx 10\n"},
		{[]string{"-k1,1", "-k2,2nr"}, "a 2 x\nb 1 y\na 1 z\nb 2 w\n", "a 2 x\na 1 z\nb 2 w\nb 1 y\n"},
		{[]string{"-t:", "-k2,2"}, "a:3:x\nb:1:y\nc:1:a\n", "b:1:y\nc:1:a\na:3:x\n"},
		{[]string{"-t:", "-k2"}, "a:3:x\nb:1:y\nc:2:z\n", "b:1:y\nc:2:z\na:3:x\n"},
		{[]string{"-k2"}, "x  b\nx a\n", "x  b\nx a\n"},
		{[]string{"-k2b"}, "x  b\nx a\n", "x a\nx  b\n"},
		{[]string{"-b", "-k2"}, "x  b\nx a\n", "x a\nx  b\n"},
		{[]string{"-k1.2"}, "ab zb\nac ya\n", "ab zb\nac ya\n"},
		// POS2's character counts from its field, as POSIX has it; busybox counted from the
		// line, which left this key empty and the lines in whole-line order.
		{[]string{"-k2.2,2.2"}, "a bz\nb ay\n", "b ay\na bz\n"},
		{[]string{"-z"}, "b\x00a\x00", "a\x00b\x00"},
		{[]string{"-d"}, "a-c\nab\n", "ab\na-c\n"},
		{[]string{"-i"}, "a\tc\nab\n", "ab\na\tc\n"},
		{[]string{"-f"}, "b\nB\na\n", "a\nB\nb\n"},
		{[]string{"-uf"}, "b\nB\na\n", "a\nB\n"},
		{[]string{"-u", "-k2"}, "a 1\nb 1\nc 2\n", "a 1\nc 2\n"},
		{[]string{"-k", ""}, "b\na\n", "a\nb\n"},
		{[]string{"-S", "10M", "-T", ".", "-m"}, "b\na\n", "a\nb\n"},
		{[]string{"-nr"}, "10\n9\n100\n", "100\n10\n9\n"},
		{[]string{}, "b\n\na", "\na\nb\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.input, append([]string{"sort"}, test.args...)...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("sort %q on %q: got %q, %q, %v; want %q", test.args, test.input, stdout, stderr, err, test.want)
		}
	}
	for _, test := range []struct {
		args          []string
		input, stderr string
		status        int
	}{
		{[]string{"-c"}, "a\nc\nb\n", "Check line 2\n", 1},
		{[]string{"-cu"}, "a\na\nb\n", "Check line 1\n", 1},
		{[]string{"-c"}, "a\nb\n", "", 0},
		{[]string{"-t::"}, "a\n", "sort: bad -t parameter\n", 2},
		{[]string{"-k0"}, "a\n", "sort: bad field specification\n", 2},
		{[]string{"-kx"}, "a\n", "sort: bad field specification\n", 2},
		{[]string{"-k1q"}, "a\n", "sort: unknown key option\n", 2},
		{[]string{"-k1u"}, "a\n", "sort: unknown sort type\n", 2},
		{[]string{"-k1V"}, "a\n", "sort: unknown sort type\n", 2},
		{[]string{"-k1,2,3"}, "a\n", "sort: unknown key option\n", 2},
		{[]string{"-ng"}, "a\nb\n", "sort: unknown sort type\n", 2},
		{[]string{"--reverse"}, "a\n", "sort: unknown option -- reverse\n", 2},
	} {
		stdout, stderr, err := runPermuted(t, view, test.input, append([]string{"sort"}, test.args...)...)
		status, _ := applets.StatusCode(err)
		if err == nil {
			status = 0
		}
		if stdout != "" || stderr != test.stderr || status != test.status {
			t.Errorf("sort %q: got %q, %q, status %d; want %q, status %d", test.args, stdout, stderr, status, test.stderr, test.status)
		}
	}
}

// -o FILE is opened once every input has been read, so `sort -o f f` sorts f in place.
func TestSort_writesTheOutputFileAfterReadingItsInput(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("b\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, err := runPermuted(t, view, "", "sort", "-o", "f", "f"); stdout != "" || stderr != "" || err != nil {
		t.Fatalf("sort -o f f: %q, %q, %v", stdout, stderr, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\nb\n" {
		t.Errorf("f holds %q, %v; want a and b sorted", data, err)
	}
}
