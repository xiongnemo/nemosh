package main

import (
	"io"
	"os"

	"golang.org/x/term"
)

// rawTerminal holds a terminal put into raw mode, and the state to put it back.
//
// Terminal state is process-global and borrowed, not owned: leaving it raw
// after exit gives the next program a terminal with no echo and no line
// discipline, which looks like a hung machine. Restoration therefore runs on
// every path out, including a panic.
type rawTerminal struct {
	descriptor int
	saved      *term.State
}

// enterRawMode switches the terminal to raw mode so the shell sees keys rather
// than lines. It reports nil when there is no terminal to switch -- a pipe, a
// file, or a test -- and the caller then reads cooked lines as before.
func enterRawMode(file *os.File) *rawTerminal {
	if file == nil {
		return nil
	}
	descriptor := int(file.Fd())
	if !term.IsTerminal(descriptor) {
		return nil
	}
	saved, err := term.MakeRaw(descriptor)
	if err != nil {
		// A terminal that refuses raw mode is not fatal: the shell still works
		// line by line, it just has no arrow keys.
		return nil
	}
	return &rawTerminal{descriptor: descriptor, saved: saved}
}

// restore puts the terminal back. Safe to call on a nil receiver, so a caller
// can defer it without first checking whether raw mode was ever entered.
func (t *rawTerminal) restore() {
	if t == nil || t.saved == nil {
		return
	}
	_ = term.Restore(t.descriptor, t.saved)
	t.saved = nil
}

// lineEditorFor builds an editor over a terminal, or nil when there is none.
//
// The editor writes to the same stream the prompt goes to, which is stderr:
// a prompt on stdout would be captured by `nemosh -i < script > out`, and the
// edited line has to appear exactly where the prompt did.
func lineEditorFor(stdin *os.File, screen io.Writer, workingDirectory string) *lineEditor {
	if stdin == nil || !term.IsTerminal(int(stdin.Fd())) {
		return nil
	}
	return newLineEditor(stdin, screen, workingDirectory)
}

// terminalFile reports the *os.File behind a reader, which is what raw mode
// needs and what an ordinary io.Reader cannot give. A test feeding a strings
// Reader gets nil here and keeps the cooked path.
func terminalFile(reader io.Reader) *os.File {
	file, ok := reader.(*os.File)
	if !ok {
		return nil
	}
	return file
}

// currentWorkingDirectory seeds file completion. The editor is built before the
// runtime, so this asks the process; runInteractiveEdited refreshes it after
// every command, which is what makes completion follow `cd`.
func currentWorkingDirectory() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return directory
}
