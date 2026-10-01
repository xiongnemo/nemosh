package applets_test

import (
	"testing"
)

// uuencode -m writes RFC 1521's form as busybox writes it, `begin-base64`, a 45-byte piece of
// base64 to a line and `====`, and uudecode reads it back. Measured against busybox-w32; -m was
// refused, and uudecode found no header in its output.
func TestUuencode_writesAndReadsBase64(t *testing.T) {
	data := "banana 3\napple 10\ncherry 2\napple 10\nDate 7\n\nelder 1\n"
	dir := writeSmallFixture(t, map[string]string{"a.txt": data})
	encoded, _, err := runSmall(t, dir, "", "uuencode", "-m", "a.txt", "a")
	want := "begin-base64 644 a\nYmFuYW5hIDMKYXBwbGUgMTAKY2hlcnJ5IDIKYXBwbGUgMTAKRGF0ZSA3Cgpl\nbGRlciAxCg==\n====\n"
	if encoded != want || err != nil {
		t.Fatalf("uuencode -m = %q, %v; want %q", encoded, err, want)
	}
	if decoded, _, err := runSmall(t, dir, encoded, "uudecode", "-o", "-"); decoded != data || err != nil {
		t.Errorf("uudecode of uuencode -m = %q, %v; want the file back", decoded, err)
	}
	if empty, _, err := runSmall(t, dir, "", "uuencode", "-m", "x"); empty != "begin-base64 644 x\n====\n" || err != nil {
		t.Errorf("uuencode -m of nothing = %q, %v", empty, err)
	}
}
