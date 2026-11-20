package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ex commands and their addresses, measured on 2026-11-21 as the motions were: 23 cases typed into
// busybox-w32's vi and vim 9.2 by a console robot. vim's where the two differ: :s takes the i
// flag, & in a replacement is the match, and ^ with g matches once, which busybox matched again
// at every place it resumed; :10 with three lines goes to the last, where busybox refused; and
// :d 2 deletes two lines, busybox ignoring the count. A backward range is busybox's `Invalid
// range`, where vim asks whether to swap it.
func TestVi_exCommandsAsBusyboxAndVimRunThem(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want, message string
		row, col                        int
	}{
		{name: ":s", text: "foo boo\n", keys: ":s/o/0/\r", want: "f0o boo"},
		{name: ":s g", text: "foo boo\n", keys: ":s/o/0/g\r", want: "f00 b00"},
		{name: ":%s g", text: "foo\nboo\n", keys: ":%s/o/0/g\r", want: "f00\nb00", row: 1,
			message: "4 substitutions on 2 lines"},
		{name: ":s i, vim's", text: "foo boo\n", keys: ":s/O/0/i\r", want: "f0o boo"},
		{name: ":s &, vim's", text: "foo\n", keys: ":s/o/[&]/g\r", want: "f[o][o]"},
		{name: ":s backreferences", text: "foo boo\n", keys: ":s/\\(o\\)\\(o\\)/\\2\\1x/\r", want: "foox boo"},
		{name: ":s with no match", text: "abc\n", keys: "l:s/x/y/\r", want: "abc", col: 1, message: "No match"},
		{name: ":s with another delimiter", text: "a/b\n", keys: ":s#/#-#\r", want: "a-b"},
		{name: ":s with an empty pattern", text: "abab\n", keys: "/b\r:s//X/\r", want: "aXab"},
		{name: ":s ^ g, vim's", text: "aaa\n", keys: ":s/^a/b/g\r", want: "baa"},
		{name: ":s \\r breaks the line, vim's", text: "a,b\n", keys: ":s/,/\\r/\r", want: "a\nb", row: 1},
		{name: ":s is one change", text: "foo\nboo\n", keys: ":%s/o/0/g\ru", want: "foo\nboo"},
		{name: ":2,3d", text: "a\nb\nc\nd\n", keys: ":2,3d\r", want: "a\nd", row: 1},
		{name: ":$", text: "a\nb\n  c\n", keys: ":$\r", want: "a\nb\n  c", row: 2, col: 2},
		{name: ":/c/d", text: "a\nb\nc\nd\n", keys: ":/c/d\r", want: "a\nb\nd", row: 2},
		{name: ":.,+1d", text: "a\nb\nc\nd\n", keys: "j:.,+1d\r", want: "a\nd", row: 1},
		{name: ":'a,'bd", text: "a\nb\nc\nd\n", keys: "jmajmbgg:'a,'bd\r", want: "a\nd", row: 1},
		{name: ":3", text: "a\nb\n  c\nd\n", keys: ":3\r", want: "a\nb\n  c\nd", row: 2, col: 2},
		{name: ":10 goes to the last line, vim's", text: "a\nb\nc\n", keys: ":10\r", want: "a\nb\nc", row: 2},
		{name: ":-1", text: "a\nb\nc\n", keys: "G:-1\r", want: "a\nb\nc", row: 1},
		{name: "a backward range", text: "a\nb\nc\n", keys: ":2,1d\r", want: "a\nb\nc", message: "Invalid range"},
		{name: "past the end", text: "a\nb\n", keys: ":5d\r", want: "a\nb", message: "Invalid range"},
		{name: "a mark not set", text: "a\nb\n", keys: ":'zd\r", want: "a\nb", message: "Mark not set"},
		{name: ":y p", text: "a\nb\n", keys: ":y\rp", want: "a\na\nb", row: 1},
		{name: ":2y P", text: "a\nb\n", keys: ":2y\rP", want: "b\na\nb"},
		{name: ":d 2, vim's", text: "a\nb\nc\nd\n", keys: ":d 2\r", want: "c\nd"},
		{name: ":%s with no match", text: "a\nb\n", keys: ":%s/x/y/\r", want: "a\nb", message: "No match"},
		{name: ":= is the last line, vim's", text: "a\nb\nc\n", keys: ":=\r", want: "a\nb\nc", message: "3"},
		{name: ":.=", text: "a\nb\nc\n", keys: "j:.=\r", want: "a\nb\nc", row: 1, message: "2"},
		{name: ":p one line", text: "a\nb\n", keys: "j:p\r", want: "a\nb", row: 1, message: "b"},
		{name: ":2yank says what it took", text: "a\nb\n", keys: ":2yank\r", want: "a\nb", message: "Yank 1 lines (2 chars) into [D]"},
		{name: "a command not here", text: "a\n", keys: ":g/a/d\r", want: "a", message: "'g' is not implemented"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
			typeVi(editor, test.keys)
			if got := viText(editor); got != test.want || editor.row != test.row || editor.col != test.col {
				t.Errorf("the buffer is %q with the cursor at %d,%d; want %q at %d,%d", got, editor.row, editor.col,
					test.want, test.row, test.col)
			}
			if test.message != "" {
				if got := viRow(t, screen, 5); got != test.message {
					t.Errorf("said %q, want %q", got, test.message)
				}
			}
		})
	}
}

// :set, busybox's options and vim's number, and :p over several lines, which waits for Enter as
// busybox's Hit_Return does.
func TestVi_setAndLists(t *testing.T) {
	type step struct{ keys, row0, last string }
	for _, test := range []struct {
		name, text string
		steps      []step
	}{
		{"number", "a\n\tb\n", []step{{":set nu\r", "  1 a", "- f.txt 1/2 50%"}, {":set nonu\r", "a", "- f.txt 1/2 50%"}}},
		{"tabstop", "\tb\n", []step{{":set ts=4\r$", "    b", "- f.txt 1/1 100%"}}},
		{"ignorecase", "a\nB\n", []step{{":set ic\r/b\r", "a", "- f.txt 2/2 100%"}}},
		{"a bad option", "a\n", []step{{":set nosuch\r", "a", "bad option: nosuch"}}},
		{"a value asked for", "a\n", []step{{":set ts?\r", "a", "tabstop=8"}}},
		{"fileformat marks the buffer changed", "a\n", []step{{":set ff=dos\r", "a", "- f.txt [Modified] 1/1 100%"}}},
		{"several lines wait for Enter", "a\nb\nc\n", []step{{":%p\r", "a", "[Hit return to continue]"}, {"\r", "a", "- f.txt 3/3 100%"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
			for _, step := range test.steps {
				typeVi(editor, step.keys)
				if got := viRow(t, screen, 0); got != step.row0 {
					t.Errorf("after %q the first row is %q, want %q", step.keys, got, step.row0)
				}
				if got := viRow(t, screen, 5); !strings.HasPrefix(got, step.last) {
					t.Errorf("after %q the last row is %q, want %q", step.keys, got, step.last)
				}
			}
		})
	}
}
