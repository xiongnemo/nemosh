package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// od, hexdump and hd dump every FILE as one stream and squeeze repeated lines, each case
// measured against busybox-w32. The offsets run on from one FILE into the next; a line the same
// as the one before is not printed and the first of a run is a `*`, unless -v; hexdump and hd
// put one before a short last line that begins as the line before it, as libbb's dump does, and
// give no length for no input. Each FILE was dumped from 0 again, and every line printed.
func TestDump_isOneSqueezedStreamAsBusyboxHasIt(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for name, content := range map[string]string{
		"one": "abc", "two": "defghijklmnopqrstu", "z64": strings.Repeat("\x00", 64),
		"z40": strings.Repeat("\x00", 40), "ab": strings.Repeat("a", 16) + strings.Repeat("b", 24), "empty": "",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zeros := strings.Repeat("  \\0", 16)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"od", "-c", "one", "two"}, "0000000   a   b   c   d   e   f   g   h   i   j   k   l   m   n   o   p\n0000020   q   r   s   t   u\n0000025\n"},
		{[]string{"hexdump", "-C", "one", "two"}, "00000000  61 62 63 64 65 66 67 68  69 6a 6b 6c 6d 6e 6f 70  |abcdefghijklmnop|\n00000010  71 72 73 74 75                                    |qrstu|\n00000015\n"},
		{[]string{"od", "-c", "z64"}, "0000000" + zeros + "\n*\n0000100\n"},
		{[]string{"od", "-c", "ab"}, "0000000" + strings.Repeat("   a", 16) + "\n0000020" + strings.Repeat("   b", 16) + "\n0000040" + strings.Repeat("   b", 8) + "\n0000050\n"},
		{[]string{"hexdump", "-C", "ab"}, "00000000  61 61 61 61 61 61 61 61  61 61 61 61 61 61 61 61  |aaaaaaaaaaaaaaaa|\n00000010  62 62 62 62 62 62 62 62  62 62 62 62 62 62 62 62  |bbbbbbbbbbbbbbbb|\n*\n00000020  62 62 62 62 62 62 62 62                           |bbbbbbbb|\n00000028\n"},
		{[]string{"hd", "z40"}, "00000000  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|\n*\n00000020  00 00 00 00 00 00 00 00                           |........|\n00000028\n"},
		{[]string{"od", "-x", "z40"}, "0000000 0000 0000 0000 0000 0000 0000 0000 0000\n*\n0000040 0000 0000 0000 0000\n0000050\n"},
		{[]string{"od", "-v", "-x", "z40"}, "0000000 0000 0000 0000 0000 0000 0000 0000 0000\n0000020 0000 0000 0000 0000 0000 0000 0000 0000\n0000040 0000 0000 0000 0000\n0000050\n"},
		{[]string{"od", "-An", "-x", "z40"}, " 0000 0000 0000 0000 0000 0000 0000 0000\n*\n 0000 0000 0000 0000\n"},
		{[]string{"hexdump", "-C", "empty"}, ""},
		{[]string{"od", "-c", "empty"}, "0000000\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("%q: %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	// A FILE that cannot be opened is named and passed over, and the stream goes on; with no
	// FILE opened there is nothing to dump and no length.
	stdout, stderr, err := runPermuted(t, view, "", "od", "-c", "one", "missing", "two")
	if !strings.HasSuffix(stdout, "0000025\n") || stderr != "od: missing: No such file or directory\n" || err == nil {
		t.Errorf("od -c one missing two: %q, %q, %v; want the stream past missing, and status 1", stdout, stderr, err)
	}
	stdout, stderr, err = runPermuted(t, view, "", "od", "-c", "missing")
	if stdout != "" || stderr != "od: missing: No such file or directory\n" || err == nil {
		t.Errorf("od -c missing: %q, %q, %v; want missing named, nothing dumped, and status 1", stdout, stderr, err)
	}
}
