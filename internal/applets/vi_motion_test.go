package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Where each motion leaves the cursor, measured on 2026-11-21 by a console robot that typed the
// keys into busybox-w32's vi and into vim 9.2 (`vim -u NONE -N`), then `i#<Esc>:wq`, and read
// where the # landed. Where the two agree that is the answer. Where they differ on a motion a
// Debian user leans on, the answer is vim's, and the case says so: busybox's b from the start of
// a line stops on the line before's last character, and its 5j with three lines left does not
// move at all. The screen cases are busybox's on a console of 30 rows; vim's ^F keeps one line
// more of overlap, which nobody's fingers know.
func TestVi_motionsLeaveTheCursorWhereBusyboxAndVimDo(t *testing.T) {
	long := ""
	for index := 1; index <= 100; index++ {
		long += "line " + itoaVi(index) + "\n"
	}
	for _, test := range []struct {
		name, text, keys string
		row, col         int
	}{
		{"w across lines", "one two three\nfour five\n", "www", 1, 0},
		{"w to an empty line", "one\n\ntwo\n", "w", 1, 0},
		{"b from a line start, vim's", "one two\nthree\n", "jb", 0, 4},
		{"b twice from a line start, vim's", "one two\nthree\n", "jbb", 0, 0},
		{"b onto an empty line", "one\n\ntwo\n", "Gb", 1, 0},
		{"b over blank lines", "one two\n\n\nthree\n", "Gb", 2, 0},
		{"b over spaces at a line start, vim's", "one two\n   three\n", "j^b", 0, 4},
		{"e across lines", "one\ntwo three\n", "ee", 1, 2},
		{"e on punctuation", "foo.bar(baz)\n", "eee", 0, 6},
		{"e from a space", "a  bc\n", "le", 0, 4},
		{"e at the end", "one two\n", "eeee", 0, 6},
		{"w on punctuation", "foo.bar(baz) x\n", "wwww", 0, 8},
		{"w at the last word", "one two\n", "www", 0, 6},
		{"W B E", "a.b c.d\ne.f\n", "WWBE", 0, 6},
		{"} {", "a\nb\n\nc\nd\n\n\ne\n", "}}{", 2, 0},
		{"} at the end", "a\nb\n", "}}", 1, 0},
		{"{ at the start", "a\nb\n", "j{", 0, 0},
		{"% from before a bracket", "x (a [b] c) y\n", "%", 0, 10},
		{"% twice", "x (a [b] c) y\n", "%%", 0, 2},
		{"% with no bracket", "abc\n", "l%", 0, 1},
		{"f ; ,", "abcabcabc\n", "fc;;,", 0, 5},
		{"t T", "abcabcabc\n", "tc$Tb", 0, 8},
		{"2fa", "xaxaxa\n", "2fa", 0, 3},
		{"f not found", "abc\n", "lfz", 0, 1},
		{"G", "  a\n\tb\n c\n", "G", 2, 1},
		{"2G", "  a\n\tb\n c\n", "2G", 1, 1},
		{"+ Enter -", "  a\n\tb\n c\n", "+\r-", 1, 1},
		{"5|", "abcdefgh\n", "5|", 0, 4},
		{"2$", "abc\ndefg\nh\n", "2$", 1, 3},
		{"j keeps the column", "abcdef\nab\nabcdef\n", "4ljj", 2, 4},
		{"$ then j", "abc\nabcdef\n", "$j", 1, 5},
		{"j past the end", "a\nb\n", "jjj", 1, 0},
		{"5j past the end, vim's", "a\nb\nc\n", "5j", 2, 0},
		{"3k past the top, vim's", "a\nb\nc\n", "j3k", 0, 0},
		{"l at the end", "abc\n", "lllll", 0, 2},
		{"10l", "abcdef\n", "10l", 0, 5},
		{"^ and 0", "   abc\n", "$^", 0, 3},
		{"^F M", long, "\x06M", 42, 0},
		{"L", long, "L", 28, 0},
		{"^F 3H", long, "\x063H", 30, 0},
		{"^D", long, "\x04", 14, 0},
		{"^F", long, "\x06", 28, 0},
		{"^F ^F ^B", long, "\x06\x06\x02", 56, 0},
		{"^E ^E", long, "\x05\x05", 2, 0},
		{"z.", long, "50Gz.", 49, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, _ := newTestVi(t, dir, "f.txt", false, 120, 30)
			typeVi(editor, test.keys)
			if editor.row != test.row || editor.col != test.col {
				line := string(editor.lines[editor.row])
				t.Errorf("the cursor is at %d,%d (%q), want %d,%d", editor.row, editor.col,
					line[:min(len(line), editor.col)]+"#"+line[min(len(line), editor.col):], test.row, test.col)
			}
			if editor.message != "" && !strings.Contains(test.name, "not found") {
				t.Errorf("said %q", editor.message)
			}
		})
	}
}

func itoaVi(number int) string {
	digits := ""
	for ; number > 0; number /= 10 {
		digits = string(rune('0'+number%10)) + digits
	}
	return digits
}
