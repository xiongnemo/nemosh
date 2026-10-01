package applets_test

import "testing"

// Under -z a NUL ends a line as a newline does, as busybox's uniq reads one, and each line is
// written with a NUL after it. A NUL ended nothing, so `printf 'a\0a\0b\0' | uniq -z` was one
// line with a NUL added. Each answer is busybox-w32's, measured.
func TestUniq_zeroReadsLinesThatEndWithNUL(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		args        []string
		stdin, want string
	}{
		{args: []string{"-z"}, stdin: "a\x00a\x00b\x00", want: "a\x00b\x00"},
		{args: []string{"-z"}, stdin: "x\x00x\x00y\x00y\x00", want: "x\x00y\x00"},
		{args: []string{"-z"}, stdin: "a\x00\x00b\x00", want: "a\x00\x00b\x00"},
		{args: []string{"-z"}, stdin: "a\nb\x00a\nb\x00c\x00", want: "a\x00b\x00a\x00b\x00c\x00"},
		{args: []string{"-zc"}, stdin: "p\x00p\x00q", want: "      2 p\x00      1 q\x00"},
		{args: []string{"-z", "-f1"}, stdin: "x y\x00x z\x00", want: "x y\x00x z\x00"},
		{args: nil, stdin: "a\na\nb\n", want: "a\nb\n"},
	} {
		if got, _, err := runSmall(t, dir, test.stdin, "uniq", test.args...); got != test.want || err != nil {
			t.Errorf("uniq %q of %q = %q, %v; want %q", test.args, test.stdin, got, err, test.want)
		}
	}
}
