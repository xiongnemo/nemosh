package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// The keys, as bytes, so the tests drive the same path a terminal does.
const (
	ctrlR = "\x12"
	ctrlG = "\x07"
	ctrlC = "\x03"
	enter = "\r"
)

// searchedLine runs a key stream against a session with history and returns the
// line that came back.
func searchedLine(t *testing.T, keys string, history ...string) string {
	t.Helper()
	_, editor := newStyledEditor(t, 80, keys, history)
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return line
}

// searchScreen stops mid-search, which is the only place the search prompt is on
// screen -- accepting or abandoning clears the row and puts the ordinary prompt
// back, so a finished line has nothing to assert against.
//
// It dispatches through the same two calls the key loop makes, in the same order,
// rather than feeding a stream: a stream that runs out submits the line.
func searchScreen(t *testing.T, keys string, history ...string) string {
	t.Helper()
	screen, editor := newStyledEditor(t, 80, "", history)
	fmt.Fprint(screen, "$ ")
	editor.redraw("$ ")
	buffer := []byte(keys)
	for len(buffer) > 0 {
		pressed, size := decodeKey(buffer)
		if size == 0 {
			t.Fatalf("undecodable key stream at %q", buffer)
		}
		buffer = buffer[size:]
		if !editor.searching() {
			if pressed.kind != keyReverseSearch {
				t.Fatalf("key %v before the search began", pressed.kind)
			}
			editor.beginHistorySearch()
			editor.drawSearch()
			continue
		}
		editor.searchKey(pressed)
		if editor.searching() {
			editor.drawSearch()
		}
	}
	var drawn strings.Builder
	for row := range 4 {
		drawn.WriteString(screen.text(row))
		drawn.WriteString("\n")
	}
	return drawn.String()
}

// Ctrl-R finds a line by any part of it, not just its first word -- which is the
// whole point. The useful memory of a command is rarely how it started.
func TestReverseSearch_findsByASubstring(t *testing.T) {
	// Given
	history := []string{"ls -al", "docker compose up -d", "git status"}

	// When: search for `comp`, then accept
	line := searchedLine(t, ctrlR+"comp"+enter+enter, history...)

	// Then
	if line != "docker compose up -d" {
		t.Fatalf("line = %q, want the matching history entry", line)
	}
}

// Enter accepts onto the line and does not run it.
//
// readline runs it immediately, which is famously how people execute the wrong
// `rm`: the match was chosen by a substring nobody re-reads before the newline.
// The first Enter leaves the search, and a second, deliberate one submits.
func TestReverseSearch_acceptsWithoutRunning(t *testing.T) {
	// Given
	history := []string{"rm -rf build", "echo safe"}

	// When: the first Enter leaves the search, then a character is typed, then a
	// second Enter submits. If the first Enter had run the line, the X could not
	// have got onto it.
	line := searchedLine(t, ctrlR+"rm"+enter+"X"+enter, history...)

	// Then
	if line != "rm -rf buildX" {
		t.Fatalf("line = %q, want the match still editable after the first Enter", line)
	}
}

// A second Ctrl-R steps to the next older match.
func TestReverseSearch_stepsToOlderMatches(t *testing.T) {
	// Given: three lines containing `git`, newest last
	history := []string{"git clone x", "git commit", "git status"}

	// When: two Ctrl-R after the pattern walks back past `git status`
	line := searchedLine(t, ctrlR+"git"+ctrlR+enter+enter, history...)

	// Then
	if line != "git commit" {
		t.Fatalf("line = %q, want the second-newest match", line)
	}
}

// At the oldest match it stops rather than wrapping, and says so the way
// readline does. Wrapping silently would make a long search loop without ever
// telling you it had.
func TestReverseSearch_reportsFailureRatherThanWrapping(t *testing.T) {
	// Given
	history := []string{"git status"}

	// When: one more step than there are matches
	drawn := searchScreen(t, ctrlR+"git"+ctrlR, history...)

	// Then
	if !strings.Contains(drawn, "failed reverse-i-search") {
		t.Fatalf("screen = %q, want the failed prompt", drawn)
	}
}

// The prompt is bash's, so the mode is unmistakable rather than a line that
// mysteriously changed.
func TestReverseSearch_showsTheSearchPrompt(t *testing.T) {
	// Given
	history := []string{"docker compose up"}

	// When
	drawn := searchScreen(t, ctrlR+"comp", history...)

	// Then
	if !strings.Contains(drawn, "(reverse-i-search)`comp':") {
		t.Fatalf("screen = %q, want bash's search prompt", drawn)
	}
}

// Ctrl-G and Ctrl-C abandon, putting back exactly what was being typed. Note
// that Ctrl-C here undoes the *search* rather than the line, which is readline's
// behaviour and leaves the least work to redo.
func TestReverseSearch_abandonsBackToWhatWasTyped(t *testing.T) {
	for _, test := range []struct {
		name string
		key  string
	}{
		{name: "Ctrl-G", key: ctrlG},
		{name: "Ctrl-C", key: ctrlC},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: half a line typed, then a search that finds something else
			line := searchedLine(t, "echo mine"+ctrlR+"git"+test.key+enter, "git status")

			// Then
			if line != "echo mine" {
				t.Fatalf("line = %q, want the typed line restored", line)
			}
		})
	}
}

// Backspace widens the pattern, and can find a different line than the narrower
// one did.
func TestReverseSearch_widensOnBackspace(t *testing.T) {
	// Given
	history := []string{"git status", "gcc main.c"}

	// When: `gi` matches `git status`; backspacing to `g` searches again from the
	// newest, which is `gcc main.c`.
	line := searchedLine(t, ctrlR+"gi\x7f"+enter+enter, history...)

	// Then
	if line != "gcc main.c" {
		t.Fatalf("line = %q, want the newest `g` match after widening", line)
	}
}

// A key the search does not want accepts the match and is then handled normally,
// in the same keystroke. Swallowing it would be a half-in, half-out mode.
func TestReverseSearch_letsAnUnwantedKeyThrough(t *testing.T) {
	// Given
	history := []string{"echo hello"}

	// When: Home leaves the search and moves to the start, then `X` is inserted
	// there rather than narrowing a pattern.
	line := searchedLine(t, ctrlR+"hello\x1b[HX"+enter, history...)

	// Then
	if line != "Xecho hello" {
		t.Fatalf("line = %q, want Home to have left the search and moved the cursor", line)
	}
}

// A pattern that matches nothing leaves the line alone: the search says failed
// and the buffer keeps whatever the last match was.
func TestReverseSearch_keepsTheLastMatchWhenNarrowingFails(t *testing.T) {
	// Given
	history := []string{"git status"}

	// When: `git` matches, `zz` does not
	line := searchedLine(t, ctrlR+"gitzz"+enter+enter, history...)
	drawn := searchScreen(t, ctrlR+"gitzz", history...)

	// Then
	if line != "git status" {
		t.Fatalf("line = %q, want the last successful match", line)
	}
	if !strings.Contains(drawn, "failed reverse-i-search)`gitzz'") {
		t.Fatalf("screen = %q, want the failed prompt naming the whole pattern", drawn)
	}
}

// With no history there is nothing to find, and the mode must still be leavable.
func TestReverseSearch_survivesAnEmptyHistory(t *testing.T) {
	// When
	line := searchedLine(t, ctrlR+"anything"+ctrlG+enter)

	// Then
	if line != "" {
		t.Fatalf("line = %q, want an empty line back", line)
	}
}
