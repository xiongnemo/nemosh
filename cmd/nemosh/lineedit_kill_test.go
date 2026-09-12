package main

import "testing"

// The kill ring, and the change of meaning `^U` took on the way.
//
// `^U` and `^W` already existed and **destroyed** what they removed, which is the half
// that makes them frightening to use. Nobody asks for a kill ring by name; they notice
// that `^U` lost something and stop pressing it.

// bufferAt builds a line with the cursor at a rune offset.
func bufferAt(text string, cursor int) *lineBuffer {
	buffer := &lineBuffer{}
	buffer.replace(text)
	buffer.cursor = cursor
	return buffer
}

func TestLineBuffer_kill(t *testing.T) {
	for _, test := range []struct {
		name       string
		text       string
		cursor     int
		kill       func(*lineBuffer) string
		wantTaken  string
		wantText   string
		wantCursor int
	}{
		{
			name: "^K takes the tail", text: "echo hello world", cursor: 5,
			kill:      func(b *lineBuffer) string { return b.killToEnd() },
			wantTaken: "hello world", wantText: "echo ", wantCursor: 5,
		},
		{
			name: "^K at the end takes nothing", text: "echo", cursor: 4,
			kill:      func(b *lineBuffer) string { return b.killToEnd() },
			wantTaken: "", wantText: "echo", wantCursor: 4,
		},
		{
			// The change of meaning: backwards to the start, not the whole line, so
			// the tail survives. That is readline's unix-line-discard.
			name: "^U takes the head and keeps the tail", text: "echo hello", cursor: 5,
			kill:      func(b *lineBuffer) string { return b.killToStart() },
			wantTaken: "echo ", wantText: "hello", wantCursor: 0,
		},
		{
			name: "^U at the start takes nothing", text: "echo", cursor: 0,
			kill:      func(b *lineBuffer) string { return b.killToStart() },
			wantTaken: "", wantText: "echo", wantCursor: 0,
		},
		{
			name: "^W takes the word before the cursor", text: "echo hello world", cursor: 16,
			kill:      func(b *lineBuffer) string { return b.killWord() },
			wantTaken: "world", wantText: "echo hello ", wantCursor: 11,
		},
		{
			// wordStart steps back over trailing blanks, so ^W after `echo   `
			// removes the word and the blanks together.
			name: "^W steps over trailing blanks", text: "echo   ", cursor: 7,
			kill:      func(b *lineBuffer) string { return b.killWord() },
			wantTaken: "echo   ", wantText: "", wantCursor: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := bufferAt(test.text, test.cursor)
			taken := test.kill(buffer)
			if taken != test.wantTaken {
				t.Errorf("took %q, want %q", taken, test.wantTaken)
			}
			if buffer.String() != test.wantText {
				t.Errorf("the line is %q, want %q", buffer.String(), test.wantText)
			}
			if buffer.cursor != test.wantCursor {
				t.Errorf("the cursor is at %d, want %d", buffer.cursor, test.wantCursor)
			}
		})
	}
}

func TestLineBuffer_yank(t *testing.T) {
	for _, test := range []struct {
		name       string
		text       string
		cursor     int
		yank       string
		wantText   string
		wantCursor int
	}{
		{
			name: "at the end", text: "echo ", cursor: 5, yank: "hello",
			wantText: "echo hello", wantCursor: 10,
		},
		{
			name: "in the middle", text: "echo world", cursor: 5, yank: "hello ",
			wantText: "echo hello world", wantCursor: 11,
		},
		{
			name: "at the start", text: "world", cursor: 0, yank: "hello ",
			wantText: "hello world", wantCursor: 6,
		},
		{
			name: "nothing to yank", text: "echo", cursor: 4, yank: "",
			wantText: "echo", wantCursor: 4,
		},
		{
			// The cursor lands after the yanked text, so a second ^Y doubles it
			// rather than interleaving.
			name: "wide characters count as runes", text: "", cursor: 0, yank: "路径",
			wantText: "路径", wantCursor: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := bufferAt(test.text, test.cursor)
			buffer.yank(test.yank)
			if buffer.String() != test.wantText {
				t.Errorf("the line is %q, want %q", buffer.String(), test.wantText)
			}
			if buffer.cursor != test.wantCursor {
				t.Errorf("the cursor is at %d, want %d", buffer.cursor, test.wantCursor)
			}
		})
	}
}

// A kill that took nothing leaves the ring alone, so a `^K` at the end of a line does not
// silently empty it and make the next `^Y` paste nothing.
func TestKillRing_anEmptyKillDoesNotClearIt(t *testing.T) {
	ring := killRing{}
	ring.kill("something")
	ring.kill("")
	if got := ring.yank(); got != "something" {
		t.Fatalf("the ring holds %q, want %q", got, "something")
	}
}

// The round trip, which is the whole point of the pair: kill a line, put it back, and get
// what was there.
func TestKillRing_roundTrip(t *testing.T) {
	for _, test := range []struct {
		name  string
		text  string
		steps func(*lineBuffer, *killRing)
		want  string
	}{
		{
			name: "^U then ^Y restores the line", text: "echo hello",
			steps: func(b *lineBuffer, k *killRing) {
				b.moveEnd()
				k.kill(b.killToStart())
				b.yank(k.yank())
			},
			want: "echo hello",
		},
		{
			name: "^K then ^Y at another place moves the text", text: "world echo ",
			steps: func(b *lineBuffer, k *killRing) {
				b.cursor = 0
				k.kill(b.killToEnd()) // takes everything
				b.yank(k.yank())
			},
			want: "world echo ",
		},
		{
			name: "^W then ^Y swaps two words", text: "world hello",
			steps: func(b *lineBuffer, k *killRing) {
				b.moveEnd()
				k.kill(b.killWord()) // takes "hello"
				b.moveHome()
				b.yank(k.yank())
				b.yank(" ")
			},
			want: "hello world ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := bufferAt(test.text, 0)
			ring := killRing{}
			test.steps(buffer, &ring)
			if got := buffer.String(); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// The two new keys decode, and the ones beside them still do -- `^K` is 0x0b and `^Y` is
// 0x19, neither of which was bound before and both of which sit next to keys that were.
func TestDecodeKey_killAndYank(t *testing.T) {
	for _, test := range []struct {
		name  string
		bytes []byte
		want  keyKind
	}{
		{name: "^K", bytes: []byte{0x0b}, want: keyKillToEnd},
		{name: "^Y", bytes: []byte{0x19}, want: keyYank},
		{name: "^U still clears", bytes: []byte{0x15}, want: keyClearLine},
		{name: "^W still deletes a word", bytes: []byte{0x17}, want: keyDeleteWord},
		// 0x0c is next to 0x0b and must stay clear-screen.
		{name: "^L still clears the screen", bytes: []byte{0x0c}, want: keyClearScreen},
		// 0x1a is next to 0x19 and must stay end-of-input.
		{name: "^Z still ends input", bytes: []byte{0x1a}, want: keyEndOfInput},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, size := decodeKey(test.bytes)
			if got.kind != test.want {
				t.Fatalf("decodeKey(%v) = %v, want %v", test.bytes, got.kind, test.want)
			}
			if size != 1 {
				t.Fatalf("decodeKey(%v) consumed %d bytes, want 1", test.bytes, size)
			}
		})
	}
}
