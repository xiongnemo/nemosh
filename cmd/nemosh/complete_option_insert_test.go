package main

import (
	"os"
	"path/filepath"
	"testing"
)

// An option must be inserted as itself.
//
// A completed *path* beginning with a dash is rewritten to `./name`, so the
// command does not read it as options. An option begins with a dash because that
// is what an option is, and applying the same rewrite to it produced
// `ls ./--color` -- the rule for making a filename usable, applied to the one
// kind of word it was never about, and the result looked to the reader as though
// Tab had completed a filename.
func TestComplete_insertsAnOptionAsItself(t *testing.T) {
	for _, test := range []struct {
		name  string
		typed string
		want  string
	}{
		{name: "a long option", typed: "ls --c", want: "ls --color "},
		{name: "a short option", typed: "ls -a", want: "ls -a "},
		{name: "for another applet", typed: "grep --c", want: "grep --color "},
		{name: "one that needs no completing", typed: "rm -r", want: "rm -r "},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: a directory with a file in it, so a wrong fallback to paths
			// would be visible rather than empty
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, editor := newStyledEditor(t, 80, "", nil)
			editor.workingDirectory = directory

			// When
			for _, r := range test.typed {
				editor.buffer.insert(r)
			}
			editor.complete("$ ")

			// Then
			if got := editor.buffer.String(); got != test.want {
				t.Fatalf("buffer = %q, want %q", got, test.want)
			}
		})
	}
}

// The path rewrite must survive: it is still applied to a path, which is the
// case it was added for.
func TestComplete_stillRewritesADashLeadingPath(t *testing.T) {
	// Given: a file whose name begins with a dash, and a command whose options
	// do not match it
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "-1.18-windows.xml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.workingDirectory = directory

	// When
	for _, r := range "ls -1.1" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if want := "ls ./-1.18-windows.xml "; editor.buffer.String() != want {
		t.Fatalf("buffer = %q, want %q", editor.buffer.String(), want)
	}
}
