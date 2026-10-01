package applets_test

import (
	"testing"
)

// wc -L counts a tab to the next multiple of eight, as busybox and GNU do, and a carriage
// return or a form feed ends a line's width and starts the next at the left. Each answer is
// busybox-w32's, measured; a tab counted nothing, so `one<TAB>two<TAB>three` was 11 where both
// say 21.
func TestWc_longestLineCountsTabsAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		input, want string
	}{
		{input: "one\ttwo\tthree\nfour\tfive\tsix\n", want: "21\n"},
		{input: "\t\n", want: "8\n"},
		{input: "abcdefgh\tx\n", want: "17\n"},
		{input: "long line\rab\n", want: "9\n"},
		{input: "ab\r\n", want: "2\n"},
		{input: "abc\fdefgh\n", want: "5\n"},
	} {
		if got, _, err := runSmall(t, dir, test.input, "wc", "-L"); got != test.want || err != nil {
			t.Errorf("wc -L on %q = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
}
