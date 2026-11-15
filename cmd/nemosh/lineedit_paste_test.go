package main

import (
	"context"
	"strings"
	"testing"
)

// A terminal in bracketed-paste mode wraps what is pasted in CSI 200~ and 201~.
func TestDecodeKey_readsThePasteMarkers(t *testing.T) {
	for input, want := range map[string]keyKind{"\x1b[200~": keyPasteStart, "\x1b[201~": keyPasteEnd} {
		if got, size := decodeKey([]byte(input)); got.kind != want || size != len(input) {
			t.Errorf("decodeKey(%q) = %v, %d, want %v, %d", input, got.kind, size, want, len(input))
		}
	}
}

// What is pasted goes into the line as it is, and runs when Enter is pressed after it. A
// pasted newline was Enter, so a script pasted at the prompt ran a line at a time as it
// arrived, and a pasted Tab asked for a completion in the middle of it.
func TestLineEditor_insertsAPasteAsItIs(t *testing.T) {
	// Given: two lines with a tab in the second, pasted, and then Enter
	screen := newScreenModel(t, 80)
	keys := "\x1b[200~echo one\r\necho\ttwo\x1b[201~\r"
	editor := newLineEditor(strings.NewReader(keys), screen, t.TempDir())
	editor.width = func() int { return 80 }

	// When
	line, err := editor.readLine(context.Background(), "$ ")

	// Then: one submission, holding both lines and the tab
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "echo one\necho\ttwo" {
		t.Fatalf("line = %q, want both lines and the tab", line)
	}
}

// A line holding a newline is drawn over two rows, the second from the left edge, and the
// cursor is put where it is in the text: rows and columns counted from the newline, not
// from the prompt. Drawing it shorter again clears the row it no longer reaches.
func TestLineEditor_drawsALineThatHoldsANewline(t *testing.T) {
	// Given
	screen := newScreenModel(t, 40)
	editor := newLineEditor(strings.NewReader(""), screen, t.TempDir())
	editor.width = func() int { return 40 }
	screen.Write([]byte("$ "))
	editor.buffer.replace("echo a\necho bc")
	editor.buffer.moveLeft()

	// When
	editor.redraw("$ ")

	// Then
	if got := screenText(screen); got != "$ echo a\necho bc\n" {
		t.Fatalf("screen = %q", got)
	}
	if row, col := screen.cursor(); row != 1 || col != 6 {
		t.Fatalf("cursor at %d,%d, want 1,6: before the c", row, col)
	}

	// And a tab in it is counted to the tab stop the terminal moves to
	editor.buffer.replace("a\n\tb")
	editor.redraw("$ ")
	if row, col := screen.cursor(); row != 1 || col != 9 {
		t.Fatalf("cursor at %d,%d, want 1,9: after the b at the first tab stop", row, col)
	}

	// And when it is drawn as one line again
	editor.buffer.replace("ls")
	editor.redraw("$ ")
	if got := screenText(screen); got != "$ ls\n" {
		t.Fatalf("screen after shrinking = %q", got)
	}
}
