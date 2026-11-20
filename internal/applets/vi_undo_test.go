package applets

import (
	"os"
	"path/filepath"
	"testing"
)

// Undo, redo, U and the dot command, measured on 2026-11-21 as the edits were: a console robot
// typed each case into busybox-w32's vi and vim 9.2, 60 and 250 ms apart, then `i#<Esc>:wq`.
// vim's where the two differ: busybox has no redo, its ^R redrawing; it undoes a cw, an o or a
// p in two steps, leaving the change's other half; its U is undone only by u, and is taken anew
// when the cursor comes back to a line; and its dw.. took a one-letter last word too, as its dw
// did (vi_edit_test.go). Where the cursor goes is vim's: to the first line the change touched,
// or the one above where it only added lines, at the column the change began in when that is
// the line.
func TestVi_undoRedoAndRepeat(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want string
		row, col               int
	}{
		{"dw u", "one two three\n", "wdwu", "one two three", 0, 4},
		{"dd u", "a\nb\nc\n", "jddu", "a\nb\nc", 1, 0},
		{"an insert is one change", "abc\n", "A!\x1bu", "abc", 0, 2},
		{"x x u u", "abcd\n", "lxxuu", "abcd", 0, 1},
		{"x x u", "abcd\n", "lxxu", "acd", 0, 1},
		{"u past the oldest", "ab\n", "xuu", "ab", 0, 0},
		{"^R redoes, vim's", "abc\n", "xxuu\x12", "bc", 0, 0},
		{"cw is one change, vim's", "one two\n", "wcwxyz\x1bu", "one two", 0, 4},
		{"o is one change, vim's", "a\nb\n", "onew\x1bu", "a\nb", 0, 0},
		{"p is one change, vim's", "a\nb\n", "yyjpu", "a\nb", 1, 0},
		{"J u", "a\nb\n", "Ju", "a\nb", 0, 0},
		{"U", "abcd\n", "lxxU", "abcd", 0, 1},
		{"U undoes U, vim's", "abcd\n", "lxxUU", "ad", 0, 1},
		{"U after leaving the line, vim's", "abc\nd\n", "xjkU", "abc\nd", 0, 0},
		{"dw ..", "a b c d\n", "dw..", "d", 0, 0},
		{"x ..", "abcdef\n", "x..", "def", 0, 0},
		{"3x .", "abcdefgh\n", "3x.", "gh", 0, 0},
		{"3x 2.", "abcdefgh\n", "3x2.", "fgh", 0, 0},
		{"cw w .", "one two three\n", "cwX\x1bw.", "X X three", 0, 2},
		{"A! j .", "a\nb\n", "A!\x1bj.", "a!\nb!", 1, 1},
		{"dd .", "a\nb\nc\nd\n", "dd.", "c\nd", 0, 0},
		{"ix .", "abc\n", "ix\x1b.", "xxabc", 0, 0},
		{"J ., the cursor vim's", "a\nb\nc\n", "J.", "a b c", 0, 3},
		{". then u", "abcdef\n", "x.u", "bcdef", 0, 0},
		{"p .", "a\n", "yyp.", "a\na\na", 2, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, _ := newTestVi(t, dir, "f.txt", false, 120, 30)
			typeVi(editor, test.keys)
			if got := viText(editor); got != test.want || editor.row != test.row || editor.col != test.col {
				t.Errorf("the buffer is %q with the cursor at %d,%d; want %q at %d,%d", got, editor.row, editor.col,
					test.want, test.row, test.col)
			}
		})
	}
}

// [Modified] goes when undo takes the buffer back to what was last written, and comes back with
// a redo, as busybox's modified_count and vim's have it; u with nothing left to undo says so.
func TestVi_undoKnowsWhatWasWritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
	for _, step := range []struct{ keys, want string }{
		{"x", "- f.txt [Modified] 1/1 100%"},
		{"u", "- f.txt 1/1 100%"},
		{"\x12", "- f.txt [Modified] 1/1 100%"},
		{":w\r\x07", "- f.txt 1/1 100%"},
		{"u", "- f.txt [Modified] 1/1 100%"},
		{"uu", "Already at oldest change"},
	} {
		typeVi(editor, step.keys)
		if got := viRow(t, screen, 5); got != step.want {
			t.Errorf("after %q the status line is %q, want %q", step.keys, got, step.want)
		}
	}
}
