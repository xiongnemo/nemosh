package applets_test

import "testing"

// -z reads and writes lines ended by a NUL, GNU's --null-data, so `find -print0 | grep -z x |
// xargs -0` keeps a name with a newline in it whole; and a NUL there is the end of a line, so
// the input is not a binary file. -b puts each line's byte offset before it, GNU's
// --byte-offset, or with -o the match's own. Both were `unknown option`.
func TestGrep_nullDataAndByteOffsets(t *testing.T) {
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{args: []string{"-z", "a"}, input: "a1\x00b2\x00a3", want: "a1\x00a3\x00"},
		{args: []string{"--null-data", "a"}, input: "a1\x00b2\x00a3", want: "a1\x00a3\x00"},
		{args: []string{"-zc", "a"}, input: "a1\x00b2\x00a3\x00", want: "2\n"},
		{args: []string{"-z", "x"}, input: "line\nwith x\x00y\x00", want: "line\nwith x\x00"},
		{args: []string{"-zv", "x"}, input: "x\x00y\x00", want: "y\x00"},
		{args: []string{"-b", "t"}, input: "one\ntwo\nthree\n", want: "4:two\n8:three\n"},
		{args: []string{"--byte-offset", "t"}, input: "one\ntwo\nthree\n", want: "4:two\n8:three\n"},
		{args: []string{"-bn", "t"}, input: "one\ntwo\nthree\n", want: "2:4:two\n3:8:three\n"},
		{args: []string{"-ob", "o"}, input: "one\ntwo\nthree\n", want: "0:o\n6:o\n"},
		// A line ended CRLF is two bytes longer than what is matched, and the offset counts them.
		{args: []string{"-b", "two"}, input: "one\r\ntwo\r\n", want: "5:two\n"},
		{args: []string{"-b", "-A1", "one"}, input: "one\ntwo\nthree\n", want: "0:one\n4-two\n"},
		{args: []string{"-zb", "b"}, input: "a1\x00b2\x00", want: "3:b2\x00"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.input, "grep", test.args...)
		if err != nil || stdout != test.want || stderr != "" {
			t.Errorf("grep %q over %q = %q, %v, %q; want %q", test.args, test.input, stdout, err, stderr, test.want)
		}
	}
}
