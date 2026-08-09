package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// completionOutput drives one line through the editor and returns every byte it
// wrote. The bell is a control character rather than something painted, so the
// screen model is the wrong instrument for it.
func completionOutput(t *testing.T, workingDirectory, keys string) string {
	t.Helper()
	var screen bytes.Buffer
	editor := newLineEditor(strings.NewReader(keys), &screen, workingDirectory)
	editor.width = func() int { return 60 }
	if _, err := editor.readLine(context.Background(), "$ "); err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return screen.String()
}

// An empty answer and a broken Tab look identical when both are silent, and that
// is not hypothetical: it is how a completion defect that made *every* argument
// uncompletable survived, because `cd ` doing nothing looked like an ordinary
// empty directory.
func TestComplete_ringsTheBell_whenThereIsNothingToOffer(t *testing.T) {
	for _, test := range []struct {
		name       string
		directory  func(t *testing.T) string
		keys       string
		wantBell   bool
		wantReason string
	}{
		{
			name: "cd where no subdirectory exists",
			directory: func(t *testing.T) string {
				directory := t.TempDir()
				if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return directory
			},
			keys: "cd \t\r", wantBell: true,
			wantReason: "a correct empty answer still has to be an answer",
		},
		{
			name:      "a prefix nothing matches",
			directory: func(t *testing.T) string { return t.TempDir() },
			keys:      "cat zzz\t\r", wantBell: true,
			wantReason: "no file begins with zzz",
		},
		{
			name: "cd where a subdirectory does exist",
			directory: func(t *testing.T) string {
				directory := t.TempDir()
				if err := os.Mkdir(filepath.Join(directory, "alpha"), 0o755); err != nil {
					t.Fatal(err)
				}
				return directory
			},
			keys: "cd \t\r", wantBell: false,
			wantReason: "it was inserted, which is feedback enough",
		},
		{
			name: "several candidates are listed rather than rung",
			directory: func(t *testing.T) string {
				directory := t.TempDir()
				for _, name := range []string{"alpha", "beta"} {
					if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return directory
			},
			keys: "cd \t\r", wantBell: false,
			wantReason: "the list is already on screen, so a bell would be noise",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			output := completionOutput(t, test.directory(t), test.keys)

			// Then
			if rang := strings.Contains(output, "\a"); rang != test.wantBell {
				t.Fatalf("bell rung = %v, want %v -- %s", rang, test.wantBell, test.wantReason)
			}
		})
	}
}
