package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A word the user has opened a quote in is completed inside the quote. `"My Do<TAB>` split
// at the blank, so it completed a word called `Do` and offered nothing; and a name went back
// with backslashes in front of its blanks, which inside a quote are literal. Now the word is
// read as the shell reads it, and the name goes back quoted the way the word was: closed, and
// followed by a blank, when it is a file and the only answer, as bash closes it, and left
// open when it is a directory, so the next Tab goes on inside it.
func TestLineEditor_completesInsideAnOpenedQuote(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"My Documents", "it's here", "cost $5.txt"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "My Documents", "read me.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "cost $5.txt", "a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct{ typed, want string }{
		{`ls "My Do`, `ls "My Documents/`},
		{`ls 'My Do`, `ls 'My Documents/`},
		{`ls "My Documents/re`, `ls "My Documents/read me.txt" `},
		{`ls 'My Documents/re`, `ls 'My Documents/read me.txt' `},
		{`ls "it`, `ls "it's here/`},
		{`ls 'it`, `ls 'it'\''s here/`},
		{`ls "cost`, `ls "cost \$5.txt/`},
		{`cd "My Documents" && ls "My Do`, `cd "My Documents" && ls "My Documents/`},
	} {
		t.Run(test.typed, func(t *testing.T) {
			// Given
			editor := newLineEditor(strings.NewReader(test.typed+"\t\r"), newScreenModel(t, 200), directory)
			editor.width = func() int { return 200 }

			// When
			line, err := editor.readLine(context.Background(), "$ ")

			// Then
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// A quoted tilde is not expanded, so a completed home path keeps its `~/` outside the quote:
// `"~/My Do<TAB>` becomes `~/"My Documents/`, which still names the home directory.
func TestLineEditor_keepsTheTildeOutsideTheQuote(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "My Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	editor := newLineEditor(strings.NewReader("ls \"~/My Do\t\r"), newScreenModel(t, 120), t.TempDir())
	editor.width = func() int { return 120 }
	editor.home = home

	line, err := editor.readLine(context.Background(), "$ ")

	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if want := `ls ~/"My Documents/`; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}
