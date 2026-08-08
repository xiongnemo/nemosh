package main

import "testing"

// Keys arrive as bytes, and an arrow is three of them. The decoder has to say
// "not yet" for a partial sequence rather than guess, because an escape byte on
// its own is also a real key.
func TestDecodeKey(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    string
		want     keyKind
		wantRune rune
		consumed int
	}{
		{name: "printable ascii", input: "a", want: keyRune, wantRune: 'a', consumed: 1},
		{name: "a multi-byte rune", input: "你", want: keyRune, wantRune: '你', consumed: 3},
		{name: "enter", input: "\r", want: keyEnter, consumed: 1},
		{name: "newline is enter too", input: "\n", want: keyEnter, consumed: 1},
		{name: "backspace", input: "\x7f", want: keyBackspace, consumed: 1},
		{name: "backspace as ctrl-h", input: "\x08", want: keyBackspace, consumed: 1},
		{name: "tab", input: "\t", want: keyTab, consumed: 1},
		{name: "ctrl-c", input: "\x03", want: keyInterrupt, consumed: 1},
		{name: "ctrl-d", input: "\x04", want: keyEndOfInput, consumed: 1},
		{name: "ctrl-z", input: "\x1a", want: keyEndOfInput, consumed: 1},
		{name: "ctrl-a", input: "\x01", want: keyHome, consumed: 1},
		{name: "ctrl-e", input: "\x05", want: keyEnd, consumed: 1},
		{name: "ctrl-u", input: "\x15", want: keyClearLine, consumed: 1},
		{name: "ctrl-w", input: "\x17", want: keyDeleteWord, consumed: 1},
		{name: "ctrl-l", input: "\x0c", want: keyClearScreen, consumed: 1},
		{name: "up", input: "\x1b[A", want: keyUp, consumed: 3},
		{name: "down", input: "\x1b[B", want: keyDown, consumed: 3},
		{name: "right", input: "\x1b[C", want: keyRight, consumed: 3},
		{name: "left", input: "\x1b[D", want: keyLeft, consumed: 3},
		{name: "home", input: "\x1b[H", want: keyHome, consumed: 3},
		{name: "end", input: "\x1b[F", want: keyEnd, consumed: 3},
		{name: "home, the other spelling", input: "\x1b[1~", want: keyHome, consumed: 4},
		{name: "delete", input: "\x1b[3~", want: keyDelete, consumed: 4},
		{name: "end, the other spelling", input: "\x1b[4~", want: keyEnd, consumed: 4},
		{name: "a lone escape", input: "\x1b", want: keyIncomplete},
		{name: "escape and bracket only", input: "\x1b[", want: keyIncomplete},
		{name: "an unfinished parameter", input: "\x1b[3", want: keyIncomplete},
		{name: "an incomplete rune", input: "\xe4\xbd", want: keyIncomplete},
		{name: "an unknown sequence is skipped whole", input: "\x1b[9~", want: keyUnknown, consumed: 4},
		{name: "nothing at all", input: "", want: keyIncomplete},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			key, consumed := decodeKey([]byte(test.input))

			// Then
			if key.kind != test.want {
				t.Fatalf("decodeKey(%q).kind = %v, want %v", test.input, key.kind, test.want)
			}
			if test.want == keyRune && key.value != test.wantRune {
				t.Fatalf("decodeKey(%q).value = %q, want %q", test.input, key.value, test.wantRune)
			}
			if test.want != keyIncomplete && consumed != test.consumed {
				t.Fatalf("decodeKey(%q) consumed %d, want %d", test.input, consumed, test.consumed)
			}
		})
	}
}

// A sequence split across two reads must survive, because a terminal is free to
// deliver an arrow key one byte at a time.
func TestDecodeKey_acceptsASequenceArrivingInPieces(t *testing.T) {
	// Given
	var pending []byte

	// When: the three bytes of an up arrow arrive separately
	for _, b := range []byte{0x1b, '[', 'A'} {
		pending = append(pending, b)
		key, consumed := decodeKey(pending)
		if key.kind == keyIncomplete {
			continue
		}
		// Then
		if key.kind != keyUp || consumed != 3 {
			t.Fatalf("decoded %v consuming %d, want keyUp consuming 3", key.kind, consumed)
		}
		return
	}
	t.Fatal("the sequence never completed")
}
