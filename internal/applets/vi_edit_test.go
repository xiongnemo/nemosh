package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What each edit leaves, the buffer and the cursor, measured on 2026-11-21 as the motions were
// (vi_motion_test.go): a console robot typed the keys into busybox-w32's vi and vim 9.2, then
// `i#<Esc>:wq`, and read the file. Where the two agree that is the answer; where they differ the
// case says whose it is. vim's where busybox's is a fault or a key a Debian user leans on: dW and
// cW onto a one-letter last word, which busybox deletes too, its at_eof test meant for a w that
// could not move; cw on blanks, which busybox ran on into the word after; S, busybox's not
// implemented; 5dd with three lines left, which busybox refuses; J's spaces, its count, and
// where it and P leave the cursor; and where the cursor opens, on the first line's first
// character that is not a blank. busybox's where vim's is only a setting most turn off: one space,
// not joinspaces' two, after a full stop. Timing was ruled out by typing 150 ms apart.
func TestVi_editsLeaveWhatBusyboxAndVimLeave(t *testing.T) {
	for _, test := range []struct {
		name, text, keys, want string
		row, col               int
	}{
		{"dw", "one two three\n", "dw", "two three", 0, 0},
		{"dw at a line's last word", "one two\nthree\n", "wdw", "one \nthree", 0, 3},
		{"dW onto a one-letter last word, vim's", "a.b c\n", "dW", "c", 0, 0},
		{"de", "one two\n", "de", " two", 0, 0},
		{"db", "one two\n", "$db", "one o", 0, 4},
		{"d$", "one two\n", "wd$", "one ", 0, 3},
		{"D", "three four\n", "wD", "three ", 0, 5},
		{"j after d$ keeps the column the cursor is in, vim's", "one two\nthree four\n", "wd$jwD", "one \nthree ", 1, 5},
		{"d0", "  one two\n", "0wd0", "one two", 0, 0},
		{"dG", "a\nb\nc\n", "jdG", "a", 0, 0},
		{"dgg", "a\nb\nc\n", "jdgg", "c", 0, 0},
		{"dj", "a\nb\nc\nd\n", "jdj", "a\nd", 1, 0},
		{"dd", "a\n  b\nc\n", "dd", "  b\nc", 0, 2},
		{"2dd", "a\nb\nc\n", "2dd", "c", 0, 0},
		{"5dd with three lines left, vim's", "a\nb\nc\n", "j5dd", "a", 0, 0},
		{"2dd on the last line", "a\nb\n", "j2dd", "a\nb", 1, 0},
		{"dd the only line", "a\n", "dd", "", 0, 0},
		{"d3w", "a b c d e f g\n", "d3w", "d e f g", 0, 0},
		{"2d2w", "a b c d e f g\n", "2d2w", "e f g", 0, 0},
		{"dfx dtx", "abxcdxef\n", "dfxdtx", "xef", 0, 0},
		{"d%", "a(b)c\n", "ld%", "ac", 0, 1},
		{"d}", "a\nb\n\nc\n", "d}", "\nc", 0, 0},
		{"dl dh", "abcd\n", "ldldh", "cd", 0, 0},
		{"x 3x", "abcdef\n", "x3x", "ef", 0, 0},
		{"x at the end", "abc\n", "$x", "ab", 0, 1},
		{"x on an empty line", "\nb\n", "x", "\nb", 0, 0},
		{"X 3X", "abcdef\n", "$X3X", "af", 0, 1},
		{"C", "one two\n", "wCnew\x1b", "one new", 0, 6},
		{"cw", "one two\n", "cwnew\x1b", "new two", 0, 2},
		{"cw on blanks, vim's", "a   b\n", "lcwx\x1b", "axb", 0, 1},
		{"cW onto a one-letter last word, vim's", "a.b c\n", "cWx\x1b", "x c", 0, 0},
		{"cc", "  one\ntwo\n", "ccnew\x1b", "new\ntwo", 0, 2},
		{"cc on the last line", "one\ntwo\n", "jccnew\x1b", "one\nnew", 1, 2},
		{"s", "abc\n", "sx\x1b", "xbc", 0, 0},
		{"3s", "abcdef\n", "3sx\x1b", "xdef", 0, 0},
		{"S, vim's", "one\ntwo\n", "Snew\x1b", "new\ntwo", 0, 2},
		{"yy p", "a\nb\n", "yyp", "a\na\nb", 1, 0},
		{"yy P", "a\nb\n", "jyyP", "a\nb\nb", 1, 0},
		{"yw P, the cursor vim's", "one two\n", "yw$P", "one twone o", 0, 9},
		{"dd p", "a\nb\nc\n", "ddp", "b\na\nc", 1, 0},
		{"3p", "ab\n", "yl3p", "aaaab", 0, 3},
		{"dd p at the end", "a\nb\n", "ddp", "b\na", 1, 0},
		{"Y p", "a\nb\n", "Yjp", "a\nb\na", 2, 0},
		{"registers", "a\nb\nc\n", "\"ayyj\"byyj\"ap\"bp", "a\nb\nc\na\nb", 4, 0},
		{"an uppercase register appends, vim's", "a\nb\n", "\"ayyj\"Ayy\"ap", "a\nb\na\nb", 2, 0},
		{"r 3r", "abcdef\n", "rx3ry", "yyydef", 0, 2},
		{"r past the end", "abc\n", "l5rx", "abc", 0, 1},
		{"r Enter", "abc\n", "lr\r", "a\nc", 1, 0},
		{"R", "abcdef\n", "lRxy\x1b", "axydef", 0, 2},
		{"R past the end", "ab\n", "lRxyz\x1b", "axyz", 0, 3},
		{"J, the cursor vim's", "a\nb\n", "J", "a b", 0, 1},
		{"J with blanks", "a\n   b\n", "J", "a b", 0, 1},
		{"J after a blank, vim's", "a \nb\n", "J", "a b", 0, 2},
		{"J before ), vim's", "a\n)b\n", "J", "a)b", 0, 1},
		{"J an empty line, vim's", "a\n\nb\n", "J", "a\nb", 0, 0},
		{"J onto an empty line, vim's", "\nb\n", "J", "b", 0, 0},
		{"J after a full stop, busybox's", "a.\nb\n", "J", "a. b", 0, 2},
		{"3J joins three lines, vim's", "a\nb\nc\nd\n", "3J", "a b c\nd", 0, 3},
		{"J on the last line", "a\nb\n", "jJ", "a\nb", 1, 0},
		{"~ 3~", "abCdef\n", "~3~", "ABcDef", 0, 4},
		{">> <<", "a\n\tb\n", ">>j<<", "\ta\nb", 1, 0},
		{">j", "a\nb\nc\n", ">j", "\ta\n\tb\nc", 0, 1},
		{"<< takes a tabstop of spaces", "          a\n", "<<", "  a", 0, 2},
		{"o O", "a\nb\n", "ox\x1bOy\x1b", "a\ny\nx\nb", 1, 0},
		{"ma and 'a", "a\nb\nc\n", "jmaG'a", "a\nb\nc", 1, 0},
		{"d'a", "a\nb\nc\nd\n", "jmajjd'a", "a", 0, 0},
		{"'' goes back", "a\nb\nc\n", "G''", "a\nb\nc", 0, 0},
		{"`a, vim's", "abc\nd\n", "llmaj`a", "abc\nd", 0, 2},
		{"the cursor opens on a first character that is not blank, vim's", "  one\n", "", "  one", 0, 2},
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

// busybox's yank_status, after a d or y that took something into a register and after a put:
// lines and characters, and the register, D when none was named. An empty register says so.
func TestVi_saysWhatARegisterTookAndGave(t *testing.T) {
	for _, test := range []struct{ name, text, keys, want string }{
		{"yy", "a\nb\n", "yy", "Yank 1 lines (2 chars) from [D]"},
		{"dw", "one two\n", "dw", "Delete 0 lines (4 chars) from [D]"},
		{"\"a2yy", "a\nb\n", "\"a2yy", "Yank 2 lines (4 chars) from [a]"},
		{"3p", "ab\n", "yl3p", "Put 0 lines (3 chars) from [D]"},
		{"p with nothing", "a\n", "p", "Nothing in register D"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(test.text), 0o644); err != nil {
				t.Fatal(err)
			}
			editor, screen := newTestVi(t, dir, "f.txt", false, 60, 6)
			typeVi(editor, test.keys)
			if got := viRow(t, screen, 5); got != test.want {
				t.Errorf("said %q, want %q", got, test.want)
			}
		})
	}
}

// viText is the buffer as it would be written, without the last newline.
func viText(e *viEditor) string {
	lines := make([]string, len(e.lines))
	for index, line := range e.lines {
		lines[index] = string(line)
	}
	return strings.Join(lines, "\n")
}
