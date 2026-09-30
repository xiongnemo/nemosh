package applets_test

import "testing"

// A replacement and a y string are read as busybox's copy_parsing_escapes reads them: a backslash
// escapes what follows it, so `\\` before the delimiter is a backslash and the delimiter ends the
// string -- `s/$/\\/` and `y/\\/\//` were refused as unmatched -- and \n, \t and \r are the
// characters, in y as well. An escaped & delimiter stays a literal ampersand, \0 is the whole
// match as \1 is the first group, and a y `\\` is one backslash. Each answer is busybox-w32's,
// measured.
func TestSed_replacementAndYEscapesAreBusyboxs(t *testing.T) {
	for _, test := range []struct {
		input  string
		script string
		want   string
	}{
		{"a/b\n", `s/$/\\/`, `a/b\` + "\n"},
		{"a/b\n", `s/a/x\\/`, `x\/b` + "\n"},
		{"a/b\n", `s|/|\\|`, `a\b` + "\n"},
		{"a/b\n", `s/a/b\/c/`, "b/c/b\n"},
		{`a\b\c` + "\n", `s/\\/\//g`, "a/b/c\n"},
		{`a\b\c` + "\n", `y/\\/\//`, "a/b/c\n"},
		{`a\b\c` + "\n", `y,\\,/,`, "a/b/c\n"},
		{"a\n", `s/$/\r/`, "a\r\n"},
		{"a\n", `s/a/x\ty\nz\rw/`, "x\ty\nz\rw\n"},
		{"abc\n", `y/abc/\n\t\r/`, "\n\t\r\n"},
		{"789\n", `s&8&\&&`, "7&9\n"},
		{"789\n", `s1\(8\)1\1\11`, "7119\n"},
		{"a\n", `s/a/\&/`, "&\n"},
		{"abc\n", `s/b/[\0]/`, "a[b]c\n"},
		{"abc\n", `s/\(b\)/[\1\0]/`, "a[bb]c\n"},
		{"a\n", `s/a/\\n/`, `\n` + "\n"},
		{"a\n", `snanxn`, "x\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, test.input, "sed", test.script)
		if err != nil || stdout != test.want {
			t.Errorf("sed %q on %q = %q (stderr %q, err %v), want %q", test.script, test.input, stdout, stderr, err, test.want)
		}
	}
}
