package main

import "testing"

// Undo is readline's: C-_ and C-x C-u take back the last change to the line, characters
// typed one after another joined twenty to one change, a key that only moves the cursor no
// change at all; M-r takes back every change. A line recalled from history starts its own.
// Each answer is readline's, as bash 5.3 binds the keys.
func TestLineEditor_undoIsReadlines(t *testing.T) {
	for _, test := range []struct {
		name, keys, want string
		history          []string
	}{
		{name: "what was typed is one change", keys: "echo hi\x1f\r", want: ""},
		{name: "a backspace taken back", keys: "echo hi\x08\x1f\r", want: "echo hi"},
		{name: "a move parts two runs of typing", keys: "echo\x01x\x1f\r", want: "echo"},
		{name: "twenty typed characters to a change", keys: "aaaaaaaaaaaaaaaaaaaaaaaaa\x1f\r", want: "aaaaaaaaaaaaaaaaaaaa"},
		{name: "C-x C-u is undo too", keys: "echo hi\x18\x15\r", want: ""},
		{name: "a kill taken back", keys: "echo hello\x17\x1f\r", want: "echo hello"},
		{name: "each undo one change further back", keys: "echo hello\x17\x08\x1f\x1f\r", want: "echo hello"},
		{name: "M-r takes back every change", keys: "echo\x08X\x17Y\x1br\r", want: ""},
		{name: "nothing to take back", keys: "\x1f\r", want: ""},
		{name: "a recalled line starts its own", keys: "\x1b[A\x1f\r", want: "ls", history: []string{"ls"}},
		{name: "a recalled line's own changes", keys: "\x1b[A -l\x1f\r", want: "ls", history: []string{"ls"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			line, err, _ := editLine(t, test.keys, test.history...)
			if err != nil || line != test.want {
				t.Fatalf("keys %q gave %q, %v; want %q", test.keys, line, err, test.want)
			}
		})
	}
}
