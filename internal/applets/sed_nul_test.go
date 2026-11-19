package applets_test

import "testing"

// sed -z reads and writes lines ended by a NUL, GNU's --null-data, so `find -print0 | sed -z
// s/a/b/ | xargs -0` keeps a name with a newline in it whole. As GNU's execute.c has it, the
// pattern space -- printed, joined by N, G and H, and parted by P and D -- uses the NUL, and
// what is not the pattern space, =, a, i, c and l, ends with a newline still. It was `unknown
// option -- z`.
func TestSed_nullData(t *testing.T) {
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{args: []string{"-z", "s/a/b/"}, input: "a1\x00a2\x00", want: "b1\x00b2\x00"},
		{args: []string{"--null-data", "s/x/y/"}, input: "line\nx\x00", want: "line\ny\x00"},
		{args: []string{"--zero-terminated", "-n", "2p"}, input: "a\x00b\x00c\x00", want: "b\x00"},
		{args: []string{"-z", "$!d"}, input: "a\x00b\x00c", want: "c"},
		{args: []string{"-z", "N;s/a.b/X/"}, input: "a\x00b\x00", want: "X\x00"},
		{args: []string{"-z", "-n", "N;P"}, input: "a\x00b\x00", want: "a\x00"},
		{args: []string{"-z", "="}, input: "a\x00", want: "1\na\x00"},
		{args: []string{"-z", "a\\\nend"}, input: "a\x00", want: "a\x00end\n"},
		// Without -z a NUL is a character like any other.
		{args: []string{"s/a/b/g"}, input: "a\x00a\n", want: "b\x00b\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.input, "sed", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q over %q = %q, %v (%s); want %q", test.args, test.input, stdout, err, stderr, test.want)
		}
	}
}
