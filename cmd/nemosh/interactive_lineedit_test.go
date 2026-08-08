package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Raw mode is only for a real terminal. Everything else -- a pipe, a file, a
// test -- keeps the cooked path, which is why the whole existing interactive
// suite still exercises it.
func TestEnterRawMode_declinesWhatIsNotATerminal(t *testing.T) {
	for _, test := range []struct {
		name string
		file *os.File
	}{
		{name: "nil"},
		{name: "a regular file", file: mustTempFile(t)},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			raw := enterRawMode(test.file)

			// Then
			if raw != nil {
				raw.restore()
				t.Fatal("entered raw mode on something that is not a terminal")
			}
		})
	}
}

// restore is safe on a nil receiver, so a caller can defer it before knowing
// whether raw mode was entered at all.
func TestRawTerminal_restoreIsSafeWhenNothingWasEntered(t *testing.T) {
	var raw *rawTerminal
	raw.restore()
	(&rawTerminal{}).restore()
}

func TestLineEditorFor_declinesWhatIsNotATerminal(t *testing.T) {
	// When
	editor := lineEditorFor(mustTempFile(t), &bytes.Buffer{}, t.TempDir())

	// Then
	if editor != nil {
		t.Fatal("built an editor for a file, want the cooked path")
	}
}

func TestTerminalFile_onlyAnswersForARealFile(t *testing.T) {
	if got := terminalFile(strings.NewReader("x")); got != nil {
		t.Fatalf("terminalFile(strings.Reader) = %v, want nil", got)
	}
	file := mustTempFile(t)
	if got := terminalFile(file); got != file {
		t.Fatalf("terminalFile(file) = %v, want the file", got)
	}
}

func mustTempFile(t *testing.T) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "probe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}
