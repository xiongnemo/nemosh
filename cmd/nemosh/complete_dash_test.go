package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A completed name beginning with a dash is read as options by whatever
// receives it, and escaping cannot reach that: quoting is resolved by the shell,
// and the operand/option split happens afterwards, inside the applet. Measured
// against this shell, `ls -l \-1.18-windows.xml` and `ls -l '-1.18-windows.xml'`
// both still fail. `./` is the only spelling that names the same file and runs.
//
// bash and busybox both hand back the bare name here and leave a command that
// cannot work; this is a deliberate divergence, and a small one.
func TestComplete_prefixesADashLeadingOperand(t *testing.T) {
	for _, test := range []struct {
		name  string
		keys  string
		files []string
		want  string
	}{
		{
			name:  "one candidate",
			keys:  "ls -1.1\t\r",
			files: []string{"-1.18-windows.xml"},
			want:  `ls ./-1.18-windows.xml `,
		},
		// A prefix no option matches, so the fallback to paths is what answers.
		// A bare `-` would not do: cat takes -n, and an option that matches wins
		// over a file, which is the rule and is tested next door.
		{
			name:  "a dash-leading file",
			keys:  "cat -o\t\r",
			files: []string{"-only.txt"},
			want:  `cat ./-only.txt `,
		},
		{
			name:  "a shared prefix is prefixed once",
			keys:  "cat -a\t\r",
			files: []string{"-alpha.txt", "-alpine.txt"},
			want:  `cat ./-alp`,
		},
		{
			name:  "an ordinary name is untouched",
			keys:  "cat no\t\r",
			files: []string{"notes.txt"},
			want:  "cat notes.txt ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			directory := t.TempDir()
			for _, name := range test.files {
				if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// When
			screen := newScreenModel(t, 70)
			editor := newLineEditor(strings.NewReader(test.keys), screen, directory)
			editor.width = func() int { return 70 }
			line, err := editor.readLine(context.Background(), "$ ")
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}

			// Then
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// Continuing from an already-prefixed word must not prefix it again, and a name
// reached through a directory was never ambiguous to begin with.
func TestComplete_doesNotPrefixTwice(t *testing.T) {
	// Given
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"-alpha.txt", filepath.Join("sub", "-inner.txt")} {
		if err := os.WriteFile(filepath.Join(directory, path), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name string
		keys string
		want string
	}{
		{name: "already prefixed", keys: "cat ./-al\t\r", want: "cat ./-alpha.txt "},
		{name: "inside a directory", keys: "cat sub/-in\t\r", want: "cat sub/-inner.txt "},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			screen := newScreenModel(t, 70)
			editor := newLineEditor(strings.NewReader(test.keys), screen, directory)
			editor.width = func() int { return 70 }
			line, err := editor.readLine(context.Background(), "$ ")
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}

			// Then
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// A command word is never rewritten: `./name` there would mean run that file,
// which is a different request from running the applet of that name.
func TestComplete_leavesACommandWordAlone(t *testing.T) {
	// Given
	directory := t.TempDir()

	// When
	screen := newScreenModel(t, 70)
	editor := newLineEditor(strings.NewReader("ech\t\r"), screen, directory)
	editor.width = func() int { return 70 }
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}

	// Then
	if line != "echo " {
		t.Fatalf("line = %q, want %q", line, "echo ")
	}
}
