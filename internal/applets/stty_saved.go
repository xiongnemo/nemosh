package applets

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// `stty -g` is the console's state as one word, which `stty WORD` puts back, as GNU's prints
// its termios settings: a script keeps it and gives it back, and never reads it. atuin's bash
// init saves the terminal so at its start and restores it around its search. Here the word is
// the console's input mode and output mode, in hex, joined by a colon -- `1f7:7` -- the output's
// a dash when no output was a console, since `$(stty -g)` takes standard output. busybox-w32's
// stty has -g and prints busybox's termios fields, which its Windows console cannot set.

// sttySavedWord is the word -g prints.
func sttySavedWord(in, out uint32, hasOut bool) string {
	if !hasOut {
		return strconv.FormatUint(uint64(in), 16) + ":-"
	}
	return strconv.FormatUint(uint64(in), 16) + ":" + strconv.FormatUint(uint64(out), 16)
}

// parseSttySaved reads a word -g printed, and false for any other setting.
func parseSttySaved(word string) (in, out uint32, hasOut, ok bool) {
	inText, outText, found := strings.Cut(word, ":")
	if !found || inText == "" || outText == "" {
		return 0, 0, false, false
	}
	value, err := strconv.ParseUint(inText, 16, 32)
	if err != nil {
		return 0, 0, false, false
	}
	if outText == "-" {
		return uint32(value), 0, false, true
	}
	output, err := strconv.ParseUint(outText, 16, 32)
	if err != nil {
		return 0, 0, false, false
	}
	return uint32(value), uint32(output), true, true
}

// sttyOutputConsole is the first of standard output and standard error that is a console,
// whose mode -g saves with the input's.
func sttyOutputConsole() (*os.File, bool) {
	for _, file := range []*os.File{os.Stdout, os.Stderr} {
		if term.IsTerminal(int(file.Fd())) {
			return file, true
		}
	}
	return nil, false
}

// writeSttySaved is -g: the input console's mode and the output's, as one word.
func writeSttySaved(out io.Writer, input *os.File) error {
	in, err := consoleMode(input)
	if err != nil {
		return err
	}
	var output uint32
	console, hasOut := sttyOutputConsole()
	if hasOut {
		if output, err = consoleMode(console); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, sttySavedWord(in, output, hasOut))
	return err
}

// restoreSttySaved puts back what a -g word saved.
func restoreSttySaved(input *os.File, in, out uint32, hasOut bool) error {
	if err := setConsoleMode(input, in); err != nil {
		return err
	}
	if console, ok := sttyOutputConsole(); ok && hasOut {
		return setConsoleMode(console, out)
	}
	return nil
}
