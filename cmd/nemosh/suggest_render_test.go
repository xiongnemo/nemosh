package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// styledEditor is the editor as a user gets it: colour on, suggestions on. It
// runs a whole line, so it is for asserting what was submitted.
func styledEditor(t *testing.T, width int, keys string, history ...string) (*screenModel, *lineEditor, string) {
	t.Helper()
	screen, editor := newStyledEditor(t, width, keys, history)
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return screen, editor, line
}

// typedScreen stops mid-line, which is the only place a suggestion exists.
//
// It types into the buffer and redraws after each key, exactly as readLine does,
// rather than feeding a stream -- a stream that runs out submits the line and
// prints a newline, which moves the cursor off the row every assertion here is
// about.
func typedScreen(t *testing.T, width int, typed string, history ...string) (*screenModel, *lineEditor) {
	t.Helper()
	screen, editor := newStyledEditor(t, width, "", history)
	fmt.Fprint(screen, "$ ")
	editor.redraw("$ ")
	for _, r := range typed {
		editor.buffer.insert(r)
		editor.redraw("$ ")
	}
	return screen, editor
}

func newStyledEditor(t *testing.T, width int, keys string, history []string) (*screenModel, *lineEditor) {
	t.Helper()
	screen := newScreenModel(t, width)
	editor := newLineEditor(strings.NewReader(keys), screen, t.TempDir())
	editor.width = func() int { return width }
	editor.styling = theme{enabled: true, colours: defaultPalette()}
	for _, entry := range history {
		editor.remember(entry)
	}
	// An empty PATH, settled. Without this the index is still building and every
	// name is undetermined, which is the right answer in production and the wrong
	// one for a test that wants to know how a name it can name is drawn.
	editor.commands.path.refresh("")
	waitForPathIndex(t, editor.commands.path)
	return screen, editor
}

func waitForPathIndex(t *testing.T, index *pathIndex) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, ready := index.has("anything"); ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the PATH index never became ready")
		}
		time.Sleep(time.Millisecond)
	}
}

// The suggestion is drawn after what was typed, in the suggestion colour, and
// the cursor stays where the typing ended.
//
// That last clause is the one worth having. Text drawn past the cursor occupies
// columns the buffer does not know about, and this editor has had two defects
// where a column count was off by exactly that kind of unaccounted width. A byte
// assertion cannot see it; the cursor position can.
func TestSuggestion_isDrawnGreyAfterTheCursor(t *testing.T) {
	// Given / When: `ec` with `echo hello` already run
	screen, _ := typedScreen(t, 40, "ec", "echo hello")

	// Then: the row reads as the whole suggested line...
	if got := screen.text(0); got != "$ echo hello" {
		t.Fatalf("row 0 = %q, want the suggestion drawn after the typed text", got)
	}
	// ...the typed part is not grey...
	if style := screen.styleAt(0, 2); style == "90" {
		t.Fatalf("typed text at column 2 is drawn in the suggestion colour (%s)", style)
	}
	// ...the un-typed part is...
	if style := screen.styleAt(0, 4); style != "90" {
		t.Fatalf("suggested text at column 4 has style %q, want the suggestion colour", style)
	}
	if got := screen.styledRun(0, 4); got != "ho hello" {
		t.Fatalf("the grey run is %q, want the part that was not typed", got)
	}
	// ...and the cursor is after `ec`, not after the suggestion.
	if row, col := screen.cursor(); row != 0 || col != 4 {
		t.Fatalf("cursor at (%d,%d), want (0,4) -- just past what was typed", row, col)
	}
}

// Enter submits what was typed. The suggestion is never in the buffer, so this
// is a property of the design rather than a rule the key handler has to keep.
func TestSuggestion_isNotSubmittedByEnter(t *testing.T) {
	// Given / When
	_, _, line := styledEditor(t, 40, "ec\r", "echo hello")

	// Then
	if line != "ec" {
		t.Fatalf("line = %q, want only what was typed", line)
	}
}

// Right at the end of the line takes it, because there is nothing there to move
// onto. Anywhere else Right still moves.
func TestSuggestion_isAcceptedByRightArrow(t *testing.T) {
	for _, test := range []struct {
		name string
		keys string
		want string
	}{
		{name: "right at the end accepts", keys: "ec\033[C\r", want: "echo hello"},
		{name: "end accepts too", keys: "ec\033[F\r", want: "echo hello"},
		{name: "right in the middle still moves", keys: "ec\033[D\033[C\r", want: "ec"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, _, line := styledEditor(t, 40, test.keys, "echo hello")

			// Then
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// A command name is suggested with no history at all, which is the state a fresh
// session is in and the state a history-only engine has nothing to say in.
func TestSuggestion_fallsBackToCommandNames(t *testing.T) {
	// Given / When
	screen, _ := typedScreen(t, 40, "mkd")

	// Then
	if got := screen.text(0); got != "$ mkdir" {
		t.Fatalf("row 0 = %q, want a command name suggested from an empty history", got)
	}
	if got := screen.styledRun(0, 5); got != "ir" {
		t.Fatalf("the grey run is %q, want the part not typed", got)
	}
}

// History wins over a command name, because a line already run is a line meant.
func TestSuggestion_prefersHistoryOverCommandNames(t *testing.T) {
	// Given / When: `mkd` could be `mkdir`, but this session ran something longer
	screen, _ := typedScreen(t, 40, "mkd", "mkdir reports")

	// Then
	if got := screen.text(0); got != "$ mkdir reports" {
		t.Fatalf("row 0 = %q, want the remembered line", got)
	}
}

// Nothing is suggested where there is nothing to go on: an empty line, or a line
// that has just ended a word.
func TestSuggestion_staysQuietWithNothingToGuessFrom(t *testing.T) {
	for _, test := range []struct {
		name string
		keys string
		want string
	}{
		{name: "an empty line", keys: "", want: "$"},
		{name: "just after a blank", keys: "echo ", want: "$ echo"},
		{name: "no candidate matches", keys: "zzq", want: "$ zzq"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			screen, _ := typedScreen(t, 40, test.keys, "echo hello")

			// Then
			if got := screen.text(0); got != test.want {
				t.Fatalf("row 0 = %q, want %q", got, test.want)
			}
		})
	}
}

// A suggestion must never push the line onto another row. Truncating to the
// columns left is what keeps every number the redraw computes from the buffer --
// where the cursor goes, how many rows to climb next time -- exactly what it was
// before suggestions existed.
func TestSuggestion_neverWraps(t *testing.T) {
	// Given: a narrow terminal and a long remembered line
	screen, _ := typedScreen(t, 20, "echo", "echo a very long remembered command line")

	// Then
	if rows := screen.rowCount(); rows != 1 {
		t.Fatalf("the line took %d rows, want 1 -- a suggestion may not wrap", rows)
	}
	if row, col := screen.cursor(); row != 0 || col != 6 {
		t.Fatalf("cursor at (%d,%d), want (0,6) -- just past `echo`", row, col)
	}
}
