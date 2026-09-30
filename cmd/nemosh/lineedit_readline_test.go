package main

import "testing"

// The keys busybox's editor binds that this one did not, C-b, C-f, C-p and C-n, and the
// readline commands bash binds that busybox has not got: C-t, M-. and M-_, M-u, M-l and M-c.
// Each answer was measured in bash 5.3.
func TestLineEditor_readlineKeys(t *testing.T) {
	for _, test := range []struct {
		name, keys, want string
		history          []string
	}{
		{name: "C-b and C-f move as the arrows do", keys: "ec\x02x\x06y\r", want: "excy"},
		{name: "C-p is the line before", keys: "\x10\r", want: "two", history: []string{"one", "two"}},
		{name: "C-p twice, then C-n", keys: "\x10\x10\x0e\r", want: "two", history: []string{"one", "two"}},
		{name: "C-t at the end swaps the two before it", keys: "ab\x14\r", want: "ba"},
		{name: "C-t drags the character before over the one at", keys: "abc\x02\x14\r", want: "acb"},
		{name: "C-t at the start does nothing", keys: "ab\x01\x14\r", want: "ab"},
		{name: "M-. is the last word of the line before", keys: "x \x1b.\r", want: "x b2", history: []string{"echo a1", "echo b2"}},
		{name: "M-_ is M-.", keys: "x \x1b_\r", want: "x b2", history: []string{"echo a1", "echo b2"}},
		{name: "M-. again reaches further back", keys: "x \x1b.\x1b.\r", want: "x a1", history: []string{"echo a1", "echo b2"}},
		{name: "past the oldest line it puts in nothing", keys: "x \x1b.\x1b.\x1b.\r", want: "x ", history: []string{"echo a1", "echo b2"}},
		{name: "another key between starts again", keys: "x \x1b.y\x1b.\r", want: "x b2yb2", history: []string{"echo a1", "echo b2"}},
		{name: "the word is the shell's", keys: "x \x1b.\r", want: `x "a b"`, history: []string{`echo "a b"`}},
		{name: "M-u upper-cases a word at a time", keys: "hello world\x01\x1bu\x1bu\r", want: "HELLO WORLD"},
		{name: "M-l and M-c", keys: "Hello WORLD\x01\x1bl\x1bc\r", want: "hello World"},
		{name: "M-c from inside a word starts a word there", keys: "hello\x02\x02\x1bc\r", want: "helLo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := pagedLine(t, test.keys, test.history...); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
