package applets_test

import "testing"

// In a regular expression the delimiter inside a bracket expression is a member of it, as
// busybox's index_of_next_unescaped_regexp_delim scans one: `s/[/]/_/` replaces a slash, where
// the pattern ended at the bracket's slash and `_` was taken for a command. A `]` right after the
// `[` or `[^` is a member, and the escaped delimiter is the delimiter, in a bracket or out. The
// replacement and y are scanned plainly. A missing delimiter is busybox's `unmatched '/'`. Each
// answer is busybox-w32's, measured.
func TestSed_aDelimiterInABracketIsAMember(t *testing.T) {
	for _, test := range []struct {
		input string
		args  []string
		want  string
	}{
		{"a/b\n", []string{"s/[/]/_/"}, "a_b\n"},
		{"a/b\n", []string{"-n", "/[/]/p"}, "a/b\n"},
		{"a]/b\n", []string{"s/[]/]/_/g"}, "a__b\n"},
		{"a^/b\n", []string{"s/[^]/]/_/g"}, "__/_\n"},
		{"a/b\n", []string{`s/[\/]/_/`}, "a_b\n"},
		{`a\b` + "\n", []string{`s/[\/]/_/`}, `a\b` + "\n"},
		{"a|b\n", []string{"s|[|]|_|"}, "a_b\n"},
		{"x:y\n", []string{"s/[[:punct:]]/_/"}, "x_y\n"},
		{"a[b\n", []string{`s/\[/_/`}, "a_b\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.input, "sed", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q on %q = %q (stderr %q, err %v), want %q", test.args, test.input, stdout, stderr, err, test.want)
		}
	}
	for _, script := range []string{"s/[/x/", "s/a/b", "y/a/b"} {
		if _, _, err := runAppletWithInput(t, "a\n", "sed", script); err == nil || err.Error() != "unmatched '/'" {
			t.Errorf("sed %q: %v, want unmatched '/'", script, err)
		}
	}
}
