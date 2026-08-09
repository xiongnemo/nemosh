package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A line longer than the terminal wraps, and once it does `\r` returns to the
// start of the *current* row rather than the row the prompt is on. Redrawing
// from there writes the line over the wrong rows.
//
// The editor therefore has to know the width and count rows. These tests read
// the bytes it emits, with the width injected, so the behaviour is checked
// without a terminal.
func editorWithWidth(t *testing.T, columns int, keys string) (*lineEditor, *bytes.Buffer) {
	t.Helper()
	var screen bytes.Buffer
	editor := newLineEditor(strings.NewReader(keys), &screen, t.TempDir())
	editor.width = func() int { return columns }
	return editor, &screen
}

// Nothing wrapped: no vertical movement at all, which keeps the common case as
// cheap as it was.
func TestRedraw_staysOnOneRowWhenTheLineFits(t *testing.T) {
	// Given
	editor, screen := editorWithWidth(t, 80, "abc\r")

	// When
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatal(err)
	}

	// Then
	if strings.Contains(screen.String(), "\033[1A") {
		t.Fatalf("screen = %q, want no row movement for a line that fits", screen.String())
	}
}

// Past the width, the redraw has to climb back to the prompt's row before it
// starts writing.
func TestRedraw_returnsToThePromptRowWhenTheLineWrapped(t *testing.T) {
	// Given: width 10, prompt 2, so the line wraps after 8 characters
	editor, screen := editorWithWidth(t, 10, "abcdefghijk\r")

	// When
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatal(err)
	}

	// Then: at least one redraw moved up a row before rewriting
	if !strings.Contains(screen.String(), "\033[1A") {
		t.Fatalf("screen = %q, want the cursor moved back up to the prompt row", screen.String())
	}
}

// The cursor lands on the right row and column, not just the right column.
func TestRedraw_placesTheCursorOnTheWrappedRow(t *testing.T) {
	for _, test := range []struct {
		name    string
		width   int
		prompt  string
		typed   string
		wantRow int
		wantCol int
	}{
		{name: "fits", width: 20, prompt: "$ ", typed: "abc", wantRow: 0, wantCol: 5},
		{name: "exactly at the edge", width: 10, prompt: "$ ", typed: "abcdefgh", wantRow: 1, wantCol: 0},
		{name: "one past the edge", width: 10, prompt: "$ ", typed: "abcdefghi", wantRow: 1, wantCol: 1},
		{name: "two rows down", width: 10, prompt: "$ ", typed: "abcdefghijklmnopqr", wantRow: 2, wantCol: 0},
		{name: "a wide character does not split", width: 10, prompt: "$ ", typed: "你好你好", wantRow: 1, wantCol: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			editor, _ := editorWithWidth(t, test.width, "")
			for _, r := range test.typed {
				editor.buffer.insert(r)
			}

			// When
			row, column := editor.cursorPosition(test.prompt)

			// Then
			if row != test.wantRow || column != test.wantCol {
				t.Fatalf("cursorPosition = (%d, %d), want (%d, %d)", row, column, test.wantRow, test.wantCol)
			}
		})
	}
}

// A shrinking line must not leave its tail on the rows below, which is what
// erasing to the end of the display is for.
func TestRedraw_erasesBelowWhenTheLineShrinks(t *testing.T) {
	// Given: type past the width, then delete back under it
	editor, screen := editorWithWidth(t, 10, "abcdefghijk\x7f\x7f\x7f\x7f\x7f\r")

	// When
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatal(err)
	}

	// Then
	if !strings.Contains(screen.String(), "\033[J") {
		t.Fatalf("screen = %q, want an erase to end of display", screen.String())
	}
}

// A width the terminal cannot report must not make the editor divide by zero or
// treat every character as a new row.
func TestRedraw_survivesAnUnknownWidth(t *testing.T) {
	for _, columns := range []int{0, -1, 1} {
		// Given
		editor, _ := editorWithWidth(t, columns, "")
		editor.buffer.insert('a')

		// When / Then: must not panic, and must produce a usable position
		row, column := editor.cursorPosition("$ ")
		if row < 0 || column < 0 {
			t.Fatalf("width %d gave position (%d, %d)", columns, row, column)
		}
	}
}
