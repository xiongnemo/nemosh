package applets_test

import (
	"testing"
)

// fold counts columns as busybox's adjust_column does: a tab to the next multiple of eight, a
// backspace back one and a carriage return back to the start, and -b every byte one. -s cuts
// after the last space or tab. Each answer is busybox-w32's, measured, but for the runes: its
// build counts the bytes of a UTF-8 character as busybox does with Unicode off, where this
// counts the character once. A tab counted one column, and -b was taken and ignored.
func TestFold_countsColumnsAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{args: []string{"-w", "10"}, input: "a\tbcdefghijklmnop\n", want: "a\tbc\ndefghijklm\nnop\n"},
		{args: []string{"-b", "-w", "10"}, input: "a\tbcdefghijklmnop\n", want: "a\tbcdefghi\njklmnop\n"},
		{args: []string{"-s", "-w", "8"}, input: "ab cd\tef gh ij\n", want: "ab cd\t\nef gh ij\n"},
		{args: []string{"-w", "5"}, input: "abc\bdefghij\n", want: "abc\bdef\nghij\n"},
		{args: []string{"-w", "5"}, input: "abc\rdefghij\n", want: "abc\rdefgh\nij\n"},
		{args: []string{"-w", "1"}, input: "\tab\n", want: "\t\na\nb\n"},
		{args: []string{"-w", "4"}, input: "héllo wörld\n", want: "héll\no wö\nrld\n"},
		{args: []string{"-b", "-w", "4"}, input: "héllo wörld\n", want: "h\xc3\xa9l\nlo w\n\xc3\xb6rl\nd\n"},
	} {
		if got, stderr, err := runSmall(t, dir, test.input, "fold", test.args...); got != test.want || err != nil {
			t.Errorf("fold %q on %q = %q, %v (%s); want %q", test.args, test.input, got, err, stderr, test.want)
		}
	}
}
