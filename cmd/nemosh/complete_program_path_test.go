package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A command name with a slash in it names a file, and Tab completes it as one: `./dep<TAB>`
// finishes the script beside it. It looked the word up among command names, where nothing
// has a slash, so a program run by its path could not be completed at all. Only what could
// run is offered, as busybox's lineedit offers it (FIND_EXE_ONLY), and directories, which
// lead to one.
func TestLineEditor_completesAProgramByItsPath(t *testing.T) {
	directory := t.TempDir()
	// notes.txt holds no #! line: on Windows the execute bit is read from the first bytes as
	// well as the suffix, so a script by any name is a program.
	for name, file := range map[string]struct {
		text string
		mode os.FileMode
	}{"nightly.sh": {"#!/bin/sh\n", 0o755}, "notes.txt": {"plain text\n", 0o644}} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(file.text), file.mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{}, runtime.State{Cwd: runtime.WorkingDirectory(directory)})

	for _, test := range []struct{ typed, want string }{
		{"./n", "./nightly.sh "},
		{"./su", "./sub/"},
		{"echo hi; ./n", "echo hi; ./nightly.sh "},
	} {
		t.Run(test.typed, func(t *testing.T) {
			// Given
			editor := newLineEditor(strings.NewReader(test.typed+"\t\r"), newScreenModel(t, 120), directory)
			editor.width = func() int { return 120 }
			editor.resolve = completionResolver(rt)

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

// /tmp is the shell's own name for the temporary directory, and completes as cd reads it.
func TestLineEditor_completesUnderTmp(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "Reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(os.TempDir(), directory)
	if err != nil || strings.HasPrefix(relative, "..") {
		t.Skipf("the test directory %s is not under %s", directory, os.TempDir())
	}
	typed := "/tmp/" + filepath.ToSlash(relative) + "/Rep"
	editor := newLineEditor(strings.NewReader("ls "+typed+"\t\r"), newScreenModel(t, 400), t.TempDir())
	editor.width = func() int { return 400 }
	editor.resolve = completionResolver(runtime.New(applets.DefaultRegistry, runtime.Streams{}))

	line, err := editor.readLine(context.Background(), "$ ")

	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if want := "ls " + strings.TrimSuffix(typed, "Rep") + "Reports/"; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}
