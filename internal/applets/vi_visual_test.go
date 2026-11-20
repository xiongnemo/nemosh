package applets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Visual mode is vim's, busybox having none: what each case leaves was measured on 2026-11-21 by a
// console robot typing it into vim 9.2 (`vim -u NONE -N`), then `i#<Esc>:wq`.
func TestVi_visualModeAsVimHasIt(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want string
		row, col               int
	}{
		{"vwd", "one two three\n", "vwd", "wo three", 0, 0},
		{"vey P", "one two\n", "vey$P", "one twoneo", 0, 8},
		{"Vjd", "a\nb\nc\n", "Vjd", "c", 0, 0},
		{"V>", "a\nb\n", "V>", "\ta\nb", 0, 1},
		{"vj$d takes the line's end", "ab\ncd\nef\n", "vj$d", "ef", 0, 0},
		{"vl~", "abc\n", "vl~", "ABc", 0, 0},
		{"vlc", "abc\n", "vlcX\x1b", "Xc", 0, 0},
		{"Vjy P", "a\nb\nc\n", "VjyGP", "a\nb\na\nb\nc", 2, 0},
		{"vjJ", "a\nb\nc\n", "vjJ", "a b\nc", 0, 1},
		{"V and :s", "aa\naa\naa\n", "Vj:s/a/b/g\r", "bb\nbb\naa", 1, 0},
		{"o goes to the other end", "abcdef\n", "lvlohd", "def", 0, 0},
		{"vr", "abcd\n", "vlr*", "**cd", 0, 0},
		{"vD takes lines", "ab\ncd\n", "lvD", "cd", 0, 0},
		{"Escape leaves it", "abc\n", "vl\x1bx", "ac", 0, 1},
		{"v then V", "ab\ncd\n", "lvVd", "cd", 0, 0},
		{"backward", "one two\n", "$vbd", "one ", 0, 3},
		{"a count", "abcdef\n", "v2ld", "def", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, _ := newTestVi(t, dir, "f.txt", false, 60, 6)
			typeVi(editor, test.keys)
			if got := viText(editor); got != test.want || editor.row != test.row || editor.col != test.col {
				t.Errorf("the buffer is %q with the cursor at %d,%d; want %q at %d,%d", got, editor.row, editor.col,
					test.want, test.row, test.col)
			}
		})
	}
}

// The selection is drawn in reverse, and the status line says which visual mode it is, as vim's.
func TestVi_drawsTheSelection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("abcd\nefgh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
	typeVi(editor, "lvl")
	reversed := func(x, y int) bool {
		_, _, style, _ := screen.GetContent(x, y)
		_, _, attributes := style.Decompose()
		return attributes&tcell.AttrReverse != 0
	}
	for x, want := range []bool{false, true, true, false} {
		if got := reversed(x, 0); got != want {
			t.Errorf("column %d reversed %v, want %v", x, got, want)
		}
	}
	if got := viRow(t, screen, 5); got != "-- VISUAL --" {
		t.Errorf("the status line is %q", got)
	}
	typeVi(editor, "V")
	if !reversed(0, 0) || !reversed(3, 0) || reversed(0, 1) {
		t.Error("V did not take the whole line")
	}
	if got := viRow(t, screen, 5); got != "-- VISUAL LINE --" {
		t.Errorf("the status line is %q", got)
	}
}
