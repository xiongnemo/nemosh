package main

import (
	"strings"
	"testing"
)

// The buffer counts runes for editing and columns for drawing, and the two are
// not the same number. busybox's own backspace conflates them: `input_backspace`
// moves the cursor back one column and deletes one character, so a two-column
// CJK character loses half of itself on screen. That is the defect this type
// exists to not have.
func TestLineBuffer_editsByRuneAndMeasuresByColumn(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		runes   int
		columns int
	}{
		{name: "ascii", text: "echo", runes: 4, columns: 4},
		{name: "cjk is two columns each", text: "你好", runes: 2, columns: 4},
		{name: "mixed", text: "a你b", runes: 3, columns: 4},
		{name: "empty", text: "", runes: 0, columns: 0},
		{name: "combining marks add no column", text: "é", runes: 2, columns: 1},
		{name: "fullwidth punctuation", text: "。", runes: 1, columns: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			buffer := newLineBuffer()
			for _, r := range test.text {
				buffer.insert(r)
			}

			// Then
			if got := buffer.runeCount(); got != test.runes {
				t.Errorf("runeCount() = %d, want %d", got, test.runes)
			}
			if got := buffer.columns(); got != test.columns {
				t.Errorf("columns() = %d, want %d", got, test.columns)
			}
			if got := buffer.String(); got != test.text {
				t.Errorf("String() = %q, want %q", got, test.text)
			}
		})
	}
}

// One backspace removes one character, whatever it costs on screen.
func TestLineBuffer_backspaceRemovesAWholeCharacter(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	for _, r := range "a你好" {
		buffer.insert(r)
	}

	// When
	buffer.backspace()

	// Then
	if got := buffer.String(); got != "a你" {
		t.Fatalf("after backspace = %q, want %q", got, "a你")
	}
	if got := buffer.columns(); got != 3 {
		t.Fatalf("columns() = %d, want 3", got)
	}

	// And the one before it, which is also two columns wide.
	buffer.backspace()
	if got := buffer.String(); got != "a" {
		t.Fatalf("after second backspace = %q, want %q", got, "a")
	}
}

func TestLineBuffer_backspaceOnAnEmptyBufferIsHarmless(t *testing.T) {
	// Given
	buffer := newLineBuffer()

	// When
	buffer.backspace()

	// Then
	if got := buffer.String(); got != "" {
		t.Fatalf("String() = %q, want empty", got)
	}
}

// The cursor moves by character too, and insertion happens where it sits.
func TestLineBuffer_movesAndInsertsAtTheCursor(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	for _, r := range "你好" {
		buffer.insert(r)
	}

	// When: left past 好, insert between them
	buffer.moveLeft()
	buffer.insert('X')

	// Then
	if got := buffer.String(); got != "你X好" {
		t.Fatalf("String() = %q, want %q", got, "你X好")
	}

	// And right past 好 puts the cursor at the end again
	buffer.moveRight()
	buffer.moveRight()
	buffer.insert('!')
	if got := buffer.String(); got != "你X好!" {
		t.Fatalf("String() = %q, want %q", got, "你X好!")
	}
}

func TestLineBuffer_cursorStopsAtBothEnds(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	buffer.insert('a')

	// When
	buffer.moveLeft()
	buffer.moveLeft()
	buffer.moveLeft()
	buffer.moveRight()
	buffer.moveRight()
	buffer.moveRight()
	buffer.insert('b')

	// Then
	if got := buffer.String(); got != "ab" {
		t.Fatalf("String() = %q, want %q", got, "ab")
	}
}

// Backspace in the middle removes the character before the cursor, not the last
// one in the line.
func TestLineBuffer_backspaceAtTheCursor(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	for _, r := range "abc" {
		buffer.insert(r)
	}
	buffer.moveLeft()

	// When
	buffer.backspace()

	// Then
	if got := buffer.String(); got != "ac" {
		t.Fatalf("String() = %q, want %q", got, "ac")
	}
}

func TestLineBuffer_homeEndAndClear(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	for _, r := range "hello" {
		buffer.insert(r)
	}

	// When / Then
	buffer.moveHome()
	buffer.insert('>')
	if got := buffer.String(); got != ">hello" {
		t.Fatalf("after home = %q, want %q", got, ">hello")
	}
	buffer.moveEnd()
	buffer.insert('<')
	if got := buffer.String(); got != ">hello<" {
		t.Fatalf("after end = %q, want %q", got, ">hello<")
	}
	buffer.replace("fresh")
	if got := buffer.String(); got != "fresh" || buffer.cursorColumns() != 5 {
		t.Fatalf("after replace = %q at column %d, want %q at 5", buffer.String(), buffer.cursorColumns(), "fresh")
	}
}

// cursorColumns is what the redraw uses to place the terminal cursor, so it has
// to measure the text before the cursor rather than count characters.
func TestLineBuffer_cursorColumnsMeasuresWidthBeforeTheCursor(t *testing.T) {
	// Given
	buffer := newLineBuffer()
	for _, r := range "你好ab" {
		buffer.insert(r)
	}

	// When
	buffer.moveHome()
	buffer.moveRight()

	// Then: one CJK character to the left of the cursor is two columns.
	if got := buffer.cursorColumns(); got != 2 {
		t.Fatalf("cursorColumns() = %d, want 2", got)
	}
}

// A control character is not insertable text; the editor handles those as keys.
func TestLineBuffer_refusesControlCharacters(t *testing.T) {
	// Given
	buffer := newLineBuffer()

	// When
	for _, r := range []rune{0x00, 0x04, 0x1a, 0x1b, 0x7f} {
		buffer.insert(r)
	}

	// Then
	if got := buffer.String(); got != "" {
		t.Fatalf("String() = %q, want control characters refused", got)
	}
}

// A tab is not inserted either: it is the completion key, and a literal tab in
// the buffer would make the drawn width unknowable.
func TestLineBuffer_refusesTab(t *testing.T) {
	buffer := newLineBuffer()
	buffer.insert('\t')
	if got := buffer.String(); got != "" {
		t.Fatalf("String() = %q, want the tab refused", got)
	}
}

func TestLineBuffer_reportsWhetherItIsEmpty(t *testing.T) {
	buffer := newLineBuffer()
	if !buffer.isEmpty() {
		t.Fatal("a new buffer is not empty")
	}
	buffer.insert('x')
	if buffer.isEmpty() {
		t.Fatal("a buffer with a character in it reports empty")
	}
	buffer.backspace()
	if !buffer.isEmpty() {
		t.Fatal("a buffer emptied by backspace reports non-empty")
	}
}

func TestLineBuffer_stringIsStableAcrossReads(t *testing.T) {
	buffer := newLineBuffer()
	for _, r := range "abc" {
		buffer.insert(r)
	}
	first := buffer.String()
	second := buffer.String()
	if first != second || !strings.EqualFold(first, "abc") {
		t.Fatalf("String() = %q then %q, want a stable %q", first, second, "abc")
	}
}
