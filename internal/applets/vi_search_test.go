package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// Searching, measured on 2026-11-21 as the motions were, 23 cases typed into busybox-w32's vi and
// vim 9.2 by a console robot. vim's where the two differ: Escape abandons a / line, which busybox
// runs; ? finds a match that starts just before the cursor, which busybox's char_search skips; and
// * and # search for the word under the cursor, which busybox has not implemented. The messages
// are busybox's, vi.c:4249-4257, 2688 and 4233, and vim's E348 for a * with no word.
func TestVi_searchesAsBusyboxAndVimDo(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want, message string
		row, col                        int
	}{
		{name: "/", text: "one two\nthree two\n", keys: "/two\r", row: 0, col: 4},
		{name: "/ n", text: "one two\nthree two\n", keys: "/two\rn", row: 1, col: 6},
		{name: "/ n n wraps", text: "one two\nthree two\n", keys: "/two\rnn", row: 0, col: 4,
			message: "search hit BOTTOM, continuing at TOP"},
		{name: "? wraps", text: "one two\nthree two\n", keys: "?two\r", row: 1, col: 6,
			message: "search hit TOP, continuing at BOTTOM"},
		{name: "? N", text: "one two\nthree two\n", keys: "?two\rN", row: 0, col: 4},
		{name: "/ N", text: "a x b x c x\n", keys: "$/x\rN", row: 0, col: 10},
		{name: "not found", text: "abc\n", keys: "l/zz\r", row: 0, col: 1, message: "Pattern not found"},
		{name: "an empty pattern is the last", text: "x a x a x\n", keys: "/a\r/\r", row: 0, col: 6},
		{name: "Escape abandons it, vim's", text: "abc abc\n", keys: "/abc\x1b", row: 0, col: 0},
		{name: "a BRE", text: "one two tao\n", keys: "/t.o\rn", row: 0, col: 8},
		{name: "^", text: "at\nta\nt\n", keys: "/^t\r", row: 1, col: 0},
		{name: "$", text: "ot\nto\nt\n", keys: "/o$\r", row: 1, col: 1},
		{name: "? just before the cursor", text: "abab\n", keys: "$?b\r", row: 0, col: 1},
		{name: "? two before the cursor, vim's", text: "abab\n", keys: "$?a\r", row: 0, col: 2},
		{name: "/ past a match at the cursor", text: "ab ab\n", keys: "/ab\r", row: 0, col: 3},
		{name: "*, vim's", text: "foo bar foo\n", keys: "*", row: 0, col: 8},
		{name: "* n", text: "foo bar foo foo\n", keys: "*n", row: 0, col: 12},
		{name: "#, vim's", text: "foo bar foo\n", keys: "$#", row: 0, col: 0},
		{name: "* is a whole word", text: "foo foobar foo\n", keys: "*", row: 0, col: 11},
		{name: "* with no word", text: "\nfoo\n", keys: "*", row: 0, col: 0, message: "No string under cursor"},
		{name: "a trailing slash ends it, vim's", text: "b/ x b\n", keys: "l/b/\r", row: 0, col: 5},
		{name: "an escaped slash", text: "a/b c\n", keys: "/\\/b\r", row: 0, col: 1},
		{name: "n before any search", text: "abc\n", keys: "n", row: 0, col: 0, message: "No previous search"},
		{name: "d/", text: "one two three\n", keys: "d/thr\r", want: "three", row: 0, col: 0},
		{name: "y? and P", text: "one two three\n", keys: "$y?two\rP", want: "one two thretwo three", row: 0, col: 11},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
			typeVi(editor, test.keys)
			if editor.row != test.row || editor.col != test.col {
				t.Errorf("the cursor is at %d,%d, want %d,%d", editor.row, editor.col, test.row, test.col)
			}
			if test.want != "" && viText(editor) != test.want {
				t.Errorf("the buffer is %q, want %q", viText(editor), test.want)
			}
			if test.message != "" {
				if got := viRow(t, screen, 5); got != test.message {
					t.Errorf("said %q, want %q", got, test.message)
				}
			}
		})
	}
}
