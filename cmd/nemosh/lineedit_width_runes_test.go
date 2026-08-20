package main

import "testing"

// The width of a rune, measured in a real console rather than assumed.
//
// This used to come from a table written out by hand, and the table was wrong: U+231A WATCH and
// U+1F680 ROCKET are drawn two cells wide by both conhost and Windows Terminal and were absent
// from it, so the cursor drifted one cell for each of them on the line. Nobody found that by
// reading the table -- it took printing a ruler into a real console and looking at where the
// column ended up.
//
// The two emoji rows are therefore the point of this test. The rest are the cases the hand
// table did get right, kept so that swapping the source of the answer cannot quietly lose them.
func TestRuneColumns(t *testing.T) {
	tests := []struct {
		name string
		rune rune
		want int
	}{
		{name: "ascii", rune: 'a', want: 1},
		{name: "CJK Han", rune: '中', want: 2},
		{name: "Hiragana", rune: 'あ', want: 2},
		{name: "Katakana", rune: 'カ', want: 2},
		{name: "Hangul syllable", rune: '한', want: 2},
		{name: "Hangul Jamo", rune: 'ᄀ', want: 2},
		{name: "fullwidth A", rune: 'Ａ', want: 2},
		{name: "ideographic space", rune: '　', want: 2},
		{name: "CJK extension B", rune: '\U00020000', want: 2},
		// The two the hand table missed, both confirmed in a console.
		{name: "watch, missing before", rune: '⌚', want: 2},
		{name: "rocket, missing before", rune: '\U0001F680', want: 2},
		{name: "grinning face", rune: '\U0001F600', want: 2},
		// Zero-width: a combining mark attaches to the glyph in front of it. Windows
		// Terminal composes them; conhost draws the mark in its own cell and so
		// disagrees, which is recorded rather than followed -- every other terminal,
		// and Unicode, say zero.
		{name: "combining acute", rune: '́', want: 0},
		{name: "zero width joiner", rune: '‍', want: 0},
		{name: "variation selector 16", rune: '️', want: 0},
		{name: "NUL", rune: 0, want: 0},
		{name: "control", rune: '\x01', want: 0},
		// Ambiguous stays narrow: drawn wide only in a CJK-locale terminal, and one
		// cell is what wcwidth defaults to and what the hand table gave it.
		{name: "ambiguous section sign", rune: '§', want: 1},
		{name: "latin with acute", rune: 'é', want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := runeColumns(test.rune)

			// Then
			if got != test.want {
				t.Fatalf("runeColumns(%q) = %d, want %d", test.rune, got, test.want)
			}
		})
	}
}

// A line of CJK is twice as wide as it is long, which is the property the cursor arithmetic
// rests on: editing counts characters and drawing counts cells.
func TestTextColumns_countsCellsNotRunes(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{text: "abcd", want: 4},
		{text: "中文", want: 4},
		{text: "中a文", want: 5},
		{text: "⌚🚀", want: 4},
		{text: "é", want: 1},
	}
	for _, test := range tests {
		// When
		got := textColumns(test.text)

		// Then
		if got != test.want {
			t.Fatalf("textColumns(%q) = %d, want %d", test.text, got, test.want)
		}
	}
}
