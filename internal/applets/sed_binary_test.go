package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// -b keeps a line's carriage return as part of the line, as busybox-w32 reads its input in
// binary mode for it: `s/$/X/` puts the X after the CR, and `-i` writes a CRLF file back as
// CRLF. Without -b the CR is dropped, as busybox-w32 drops it, and `r` drops it either way, as
// busybox-w32 reads that file as text. Each answer is busybox-w32's, measured; -b was refused.
func TestSed_keepsCarriageReturnsUnderMinusB(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"crlf.txt": "a\r\nb\r\n", "more.txt": "m\r\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"s/$/X/", "crlf.txt"}, want: "aX\nbX\n"},
		{args: []string{"-b", "s/$/X/", "crlf.txt"}, want: "a\rX\nb\rX\n"},
		{args: []string{"-b", "1r more.txt", "crlf.txt"}, want: "a\r\nm\nb\r\n"},
	} {
		if stdout, stderr, err := runSmall(t, dir, "", "sed", test.args...); stdout != test.want || stderr != "" || err != nil {
			t.Errorf("sed %q = %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-i", "s/a/A/", "crlf.txt"}, want: "A\nb\n"},
		{args: []string{"-b", "-i", "s/a/A/", "crlf.txt"}, want: "A\r\nb\r\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, "crlf.txt"), []byte("a\r\nb\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, stderr, err := runSmall(t, dir, "", "sed", test.args...); err != nil {
			t.Fatalf("sed %q: %v (stderr %q)", test.args, err, stderr)
		}
		if data, _ := os.ReadFile(filepath.Join(dir, "crlf.txt")); string(data) != test.want {
			t.Errorf("sed %q left %q; want %q", test.args, data, test.want)
		}
	}
}
