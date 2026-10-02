package main

import "testing"

// The kill ring is readline's: kills straight after one another are one entry, the text in
// the order it stood, ^Y puts in the newest, and M-y straight after ^Y or M-y takes that
// back out and puts in the kill before it, round the ring; it stays turned for the next ^Y.
// Every answer is readline's, as bash 5.3 binds the keys.
func TestLineEditor_killRingIsReadlines(t *testing.T) {
	for _, test := range []struct {
		name, keys, want string
	}{
		// Two ^W straight after each other are one kill, and ^Y puts back both words.
		{name: "two ^W are one entry", keys: "echo one two\x17\x17\x19\r", want: "echo one two"},
		// A key between them makes two entries, and ^Y puts in the newest.
		{name: "a key between kills parts them", keys: "a b\x17x\x08\x17\x19\r", want: "a "},
		// M-d kills forward, and two of them join after each other.
		{name: "M-d kills forward and joins", keys: "one two three\x01\x1bd\x1bd\x05 \x19\r", want: " three one two"},
		// ^Y then M-y: the yank taken out and the kill before it put in.
		{name: "M-y after ^Y turns the ring", keys: "first\x15second\x15\x19\x1by\r", want: "first"},
		// M-y twice goes round the ring and back to the newest.
		{name: "M-y round the ring", keys: "first\x15second\x15\x19\x1by\x1by\r", want: "second"},
		// M-y without a yank straight before it does nothing.
		{name: "M-y alone does nothing", keys: "first\x15x\x1by\r", want: "x"},
		// The ring stays turned: a later ^Y puts in what M-y chose.
		{name: "the ring stays turned", keys: "first\x15second\x15\x19\x1by-\x19\r", want: "first-first"},
	} {
		t.Run(test.name, func(t *testing.T) {
			line, err, _ := editLine(t, test.keys)
			if err != nil || line != test.want {
				t.Fatalf("keys %q gave %q, %v; want %q", test.keys, line, err, test.want)
			}
		})
	}
}
