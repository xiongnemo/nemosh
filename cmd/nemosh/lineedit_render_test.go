package main

import (
	"context"
	"strings"
	"testing"
)

// These assert what the screen looks like, not which bytes were sent.
//
// Both defects a user hit were invisible to a byte assertion. A prompt measured
// 11 columns instead of 2 still emitted a well-formed `\033[NC`; a wrapped line
// still emitted a well-formed redraw. Only the numbers were wrong, and a number
// is exactly what `strings.Contains` cannot judge. Rendering the sequences and
// looking at the result is the difference between testing the shape and testing
// the effect -- the same distinction that let `times` report 215 years under a
// green test.
func renderEditor(t *testing.T, width int, prompt, keys string) *screenModel {
	t.Helper()
	screen := newScreenModel(t, width)
	editor := newLineEditor(strings.NewReader(keys), screen, t.TempDir())
	editor.width = func() int { return width }
	// Highlighting stays on -- it emits escapes, and these tests are the ones
	// that would catch an escape the screen cannot make sense of. Suggestions go
	// off: they put text on the row that these assertions are not about, and a
	// test that has to restate an unrelated feature stops testing its own.
	editor.styling.colours.suggestion = nil
	if _, err := editor.readLine(context.Background(), prompt); err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return screen
}

// The prompt-width defect, rendered: with a coloured prompt the typed text must
// begin immediately after the visible `$ `, not nine cells later.
func TestRender_typedTextFollowsAColouredPrompt(t *testing.T) {
	for _, test := range []struct {
		name   string
		prompt string
	}{
		{name: "plain", prompt: "$ "},
		{name: "coloured", prompt: "\033[1;31m$\033[0m "},
		{name: "several colours", prompt: "\033[1;34m#\033[0m \033[0;33m~\033[0m$ "},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			screen := renderEditor(t, 80, test.prompt, "echo hi\r")

			// Then: the row reads as the prompt's visible text followed directly
			// by what was typed.
			visible := promptVisibleText(test.prompt)
			if got := screen.text(0); got != visible+"echo hi" {
				t.Fatalf("row 0 = %q, want %q", got, visible+"echo hi")
			}
		})
	}
}

// Editing renders too: a backspace over a two-column character must leave no
// half of it behind, which is the busybox defect stated as a picture.
func TestRender_backspaceOverAWideCharacterLeavesNothing(t *testing.T) {
	// Given / When
	screen := renderEditor(t, 80, "$ ", "a你好\x7f\x7f\r")

	// Then
	if got := screen.text(0); got != "$ a" {
		t.Fatalf("row 0 = %q, want %q", got, "$ a")
	}
}

// The wrapping defect, rendered: a line longer than the terminal must continue
// onto the next row and stay correct there, rather than being rewritten over
// whatever row the cursor happened to be on.
func TestRender_aWrappedLineIsWholeAcrossRows(t *testing.T) {
	// Given: width 10 and a prompt of 2 leaves 8 columns on the first row
	screen := renderEditor(t, 10, "$ ", "abcdefghijkl\r")

	// Then
	if got := screen.text(0); got != "$ abcdefgh" {
		t.Fatalf("row 0 = %q, want %q", got, "$ abcdefgh")
	}
	if got := screen.text(1); got != "ijkl" {
		t.Fatalf("row 1 = %q, want %q", got, "ijkl")
	}
}

// Shrinking a wrapped line must clear the row it no longer reaches, rather than
// leaving the tail visible below.
func TestRender_shrinkingAWrappedLineClearsTheRowBelow(t *testing.T) {
	// Given: wrap onto a second row, then delete back under the width
	screen := renderEditor(t, 10, "$ ", "abcdefghijkl\x7f\x7f\x7f\x7f\x7f\x7f\r")

	// Then
	if got := screen.text(0); got != "$ abcdef" {
		t.Fatalf("row 0 = %q, want %q", got, "$ abcdef")
	}
	if got := screen.text(1); got != "" {
		t.Fatalf("row 1 = %q, want it cleared", got)
	}
}

// History recall replaces the whole line, including when the recalled one is
// shorter than what it replaces.
func TestRender_recallingAShorterLineClearsTheRest(t *testing.T) {
	// Given
	screen := newScreenModel(t, 80)
	editor := newLineEditor(strings.NewReader("\033[A\r"), screen, t.TempDir())
	editor.width = func() int { return 80 }
	editor.styling.colours.suggestion = nil
	editor.remember("hi")

	// When: type nothing, recall the short entry over the empty line
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatal(err)
	}

	// Then
	if got := screen.text(0); got != "$ hi" {
		t.Fatalf("row 0 = %q, want %q", got, "$ hi")
	}
}

// A multi-line prompt puts the edited text on its last row, at the column that
// row ends in -- not at the width of the whole prompt.
func TestRender_editsOnTheLastRowOfAMultiLinePrompt(t *testing.T) {
	// Given / When
	screen := renderEditor(t, 80, "\033[1;34m# nemo in /tmp\033[0m\n\033[1;31m$\033[0m ", "ls\r")

	// Then
	if got := screen.text(0); got != "# nemo in /tmp" {
		t.Fatalf("row 0 = %q, want the prompt's first row", got)
	}
	if got := screen.text(1); got != "$ ls" {
		t.Fatalf("row 1 = %q, want %q", got, "$ ls")
	}
}

// The cursor is where the next character would land, which is what an insertion
// in the middle depends on.
func TestRender_cursorSitsWhereTheNextCharacterGoes(t *testing.T) {
	for _, test := range []struct {
		name    string
		width   int
		keys    string
		wantRow int
		wantCol int
	}{
		{name: "after typing", width: 80, keys: "abc", wantRow: 0, wantCol: 5},
		{name: "after moving left twice", width: 80, keys: "abc\033[D\033[D", wantRow: 0, wantCol: 3},
		{name: "at home", width: 80, keys: "abc\x01", wantRow: 0, wantCol: 2},
		{name: "wrapped onto the second row", width: 10, keys: "abcdefghi", wantRow: 1, wantCol: 1},
		{name: "wrapped, then home", width: 10, keys: "abcdefghi\x01", wantRow: 0, wantCol: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			screen := newScreenModel(t, test.width)
			editor := newLineEditor(strings.NewReader(test.keys), screen, t.TempDir())
			editor.width = func() int { return test.width }

			// When: the stream ends without Enter, so the buffer is submitted
			// after every key has been drawn.
			if _, err := editor.readLine(context.Background(), "$ "); err != nil {
				t.Fatal(err)
			}

			// Then: readLine writes a newline on submit, so look at where the
			// cursor was before that by rendering the same keys again without it.
			screen = newScreenModel(t, test.width)
			editor = newLineEditor(strings.NewReader(test.keys), screen, t.TempDir())
			editor.width = func() int { return test.width }
			editor.buffer = newLineBuffer()
			editor.resetDrawState()
			replayKeys(t, editor, screen, test.keys)
			row, column := screen.cursor()
			if row != test.wantRow || column != test.wantCol {
				t.Fatalf("cursor = (%d, %d), want (%d, %d)", row, column, test.wantRow, test.wantCol)
			}
		})
	}
}

// replayKeys drives the editor's key handling without the submit that ends
// readLine, so the cursor can be inspected mid-line.
func replayKeys(t *testing.T, editor *lineEditor, screen *screenModel, keys string) {
	t.Helper()
	if _, err := screen.Write([]byte("$ ")); err != nil {
		t.Fatal(err)
	}
	pending := []byte(keys)
	for len(pending) > 0 {
		decoded, consumed := decodeKey(pending)
		if decoded.kind == keyIncomplete {
			break
		}
		pending = pending[consumed:]
		applyKeyForTest(editor, decoded)
		editor.redraw("$ ")
	}
}

func applyKeyForTest(editor *lineEditor, k key) {
	switch k.kind {
	case keyRune:
		editor.buffer.insert(k.value)
	case keyBackspace:
		editor.buffer.backspace()
	case keyLeft:
		editor.buffer.moveLeft()
	case keyRight:
		editor.buffer.moveRight()
	case keyHome:
		editor.buffer.moveHome()
	case keyEnd:
		editor.buffer.moveEnd()
	}
}

// promptVisibleText is what a person sees of a prompt, used to state the
// expectation independently of how the editor measures it.
func promptVisibleText(prompt string) string {
	var visible strings.Builder
	for index := 0; index < len(prompt); index++ {
		if prompt[index] == 0x1b {
			index = skipEscapeSequence(prompt, index) - 1
			continue
		}
		visible.WriteByte(prompt[index])
	}
	return visible.String()
}
