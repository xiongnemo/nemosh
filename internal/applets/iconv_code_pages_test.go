package applets_test

import "testing"

// iconv takes ASCII and a Windows code page by its number, `CP1252`, as busybox-w32's iconv
// names them; both were "conversion from/to ... is not supported". Each answer is
// busybox-w32's, measured.
func TestIconv_takesASCIIAndCodePageNumbers(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		args        []string
		stdin, want string
	}{
		{args: []string{"-f", "ASCII", "-t", "UTF-8"}, stdin: "abc\n", want: "abc\n"},
		{args: []string{"-f", "UTF-8", "-t", "ascii"}, stdin: "abc\n", want: "abc\n"},
		{args: []string{"-f", "CP1252", "-t", "UTF-8"}, stdin: "\xe9\n", want: "é\n"},
		{args: []string{"-f", "UTF-8", "-t", "cp437"}, stdin: "é\n", want: "\x82\n"},
		{args: []string{"-f", "CP936", "-t", "UTF-8"}, stdin: "\xc4\xe3\n", want: "你\n"},
	} {
		if got, _, err := runSmall(t, dir, test.stdin, "iconv", test.args...); got != test.want || err != nil {
			t.Errorf("iconv %q of %q = %q, %v; want %q", test.args, test.stdin, got, err, test.want)
		}
	}
	// A code page Windows has not got is refused as busybox-w32 refuses it, which names nothing.
	if _, _, err := runSmall(t, dir, "x\n", "iconv", "-f", "CP99999"); err == nil || err.Error() != "iconv_open error: Invalid argument" {
		t.Errorf("iconv -f CP99999 = %v; want busybox-w32's iconv_open error", err)
	}
}
