package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tab straight after a blank is the commonest completion gesture there is --
// `cd ` then Tab -- and it did nothing at all.
//
// The word boundary was shared with Ctrl-W, and the two want opposite things.
// Deleting the word before the cursor in `echo   ` should remove `echo`, so
// wordStart walks back over the blanks first. Completing after `cd ` must not:
// the blank has begun a new, empty word. Sharing the one boundary made the word
// under the cursor `"cd "`, which was then completed as a command name, and
// nothing is called `"cd "` -- so zero matches, and a Tab that did nothing.
//
// Only the empty word was affected, which is why this survived: `cd a` then Tab
// worked, because there the boundary lands in the same place either way.
func TestComplete_afterABlank_completesTheArgumentNotTheCommand(t *testing.T) {
	// Given
	directory := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	screen := newScreenModel(t, 60)
	editor := newLineEditor(strings.NewReader("cd \t\r"), screen, directory)
	editor.width = func() int { return 60 }
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatalf("readLine: %v", err)
	}

	// Then: the candidates are on screen somewhere.
	var painted strings.Builder
	for row := 0; row < screen.rowCount(); row++ {
		painted.WriteString(screen.text(row))
		painted.WriteString("\n")
	}
	for _, want := range []string{"alpha/", "beta/"} {
		if !strings.Contains(painted.String(), want) {
			t.Fatalf("screen never showed %q after `cd ` and Tab:\n%s", want, painted.String())
		}
	}
}

// The two boundaries are different questions, so they are asked separately.
func TestWordBoundaries_differForDeletionAndCompletion(t *testing.T) {
	for _, test := range []struct {
		name             string
		typed            string
		wantDeletionWord string
		wantCompletion   string
	}{
		{name: "mid-word", typed: "echo al", wantDeletionWord: "echo ", wantCompletion: "al"},
		{name: "just after a blank", typed: "echo ", wantDeletionWord: "", wantCompletion: ""},
		{name: "after several blanks", typed: "echo   ", wantDeletionWord: "", wantCompletion: ""},
		{name: "at the start", typed: "ec", wantDeletionWord: "", wantCompletion: "ec"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			buffer := &lineBuffer{}
			for _, r := range test.typed {
				buffer.insert(r)
			}

			// Then: completion sees an empty word after a blank...
			if got := buffer.currentWord(); got != test.wantCompletion {
				t.Fatalf("currentWord() = %q, want %q", got, test.wantCompletion)
			}

			// ...while Ctrl-W still deletes the word before the blanks.
			buffer.deleteWord()
			if got := buffer.String(); got != test.wantDeletionWord {
				t.Fatalf("after deleteWord() the buffer is %q, want %q", got, test.wantDeletionWord)
			}
		})
	}
}
