package applets_test

import "testing"

// cut --complement keeps what LIST does not name, as GNU's cut has it: `cut -d: --complement -f2`.
// It was `unknown option -- complement`.
func TestCut_complement(t *testing.T) {
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{args: []string{"-d:", "--complement", "-f2"}, input: "a:b:c:d\n", want: "a:c:d\n"},
		{args: []string{"-d:", "--complement", "-f1-2"}, input: "a:b:c\n", want: "c\n"},
		{args: []string{"-d:", "--complement", "-f2-"}, input: "a:b:c\n", want: "a\n"},
		{args: []string{"-d:", "--complement", "-f1,3"}, input: "a:b:c:d\n", want: "b:d\n"},
		{args: []string{"--complement", "-c2-3"}, input: "abcdef\n", want: "adef\n"},
		{args: []string{"--complement", "-b1,3"}, input: "abcdef\n", want: "bdef\n"},
		// A line with no delimiter is printed whole under -f, as it is without --complement.
		{args: []string{"-d:", "--complement", "-f2"}, input: "abc\n", want: "abc\n"},
		{args: []string{"--complement", "-c1-"}, input: "abc\n", want: "\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.input, "cut", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("cut %q over %q = %q, %v (%s); want %q", test.args, test.input, stdout, err, stderr, test.want)
		}
	}
}
