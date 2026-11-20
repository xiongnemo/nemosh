package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// Inserting, measured on 2026-11-21 as the motions were: 16 cases typed into busybox-w32's vi and
// vim 9.2 by a console robot. They agree on all but ^W and ^U, which busybox puts in as
// characters; vim's ^W takes the word before the cursor and ^U what was typed before it, then
// all of the line before it, and so do these.
func TestVi_insertsAsBusyboxAndVimDo(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want string
		row, col               int
	}{
		{"autoindent at Enter", "  x\n", ":set ai\rA\ry\x1b", "  x\n  y", 1, 2},
		{"autoindent a line left empty", "  x\nz\n", ":set ai\ro\x1b", "  x\n\nz", 1, 0},
		{"autoindent O", "a\n  x\n", ":set ai\rjOz\x1b", "a\n  z\n  x", 1, 2},
		{"^D takes an indent back", "\tx\n", ":set ai\ro\x04y\x1b", "\tx\ny", 1, 0},
		{"Enter on an autoindent moves it", "  x\n", ":set ai\ro\ry\x1b", "  x\n\n  y", 2, 2},
		{"expandtab", "x\n", ":set et\ri\ty\x1b", "        yx", 0, 8},
		{"^W, vim's", "foo bar\n", "A\x17\x1b", "foo ", 0, 3},
		{"^W ^W, vim's", "foo bar\n", "A\x17\x17\x1b", "", 0, 0},
		{"^W after typing, vim's", "foo\n", "A bar\x17\x1b", "foo ", 0, 3},
		{"^U what was typed, vim's", "foo\n", "Abar\x15\x1b", "foo", 0, 2},
		{"^U ^U, vim's", "foo\n", "Abar\x15\x15\x1b", "", 0, 0},
		{"R and Backspace put back", "abcd\n", "Rxy\x7f\x7fz\x1b", "zbcd", 0, 0},
		{"R past the end and Backspace", "ab\n", "lRxyz\x7f\x7f\x1b", "ax", 0, 1},
		{"Insert turns an insert to R", "abc\n", "i¤x\x1b", "xbc", 0, 0},
		{"^V puts a key in as it is", "ab\n", ":set et\ri\x16\tx\x1b", "\txab", 0, 1},
		{"Backspace joins lines", "a\nb\n", "ji\x7f\x1b", "ab", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, _ := newTestVi(t, dir, "f.txt", false, 60, 6)
			for _, r := range test.keys {
				if r == '¤' {
					r = viKeyInsert // the Insert key, which has no character
				}
				editor.key(r)
			}
			if got := viText(editor); got != test.want || editor.row != test.row || editor.col != test.col {
				t.Errorf("the buffer is %q with the cursor at %d,%d; want %q at %d,%d", got, editor.row, editor.col,
					test.want, test.row, test.col)
			}
		})
	}
}
