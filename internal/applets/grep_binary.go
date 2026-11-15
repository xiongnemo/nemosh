package applets

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// A binary file, as GNU grep tells one: a NUL in it.
//
// `grep -r foo .` in a tree with an .exe in it printed the matching "line" of the executable --
// raw bytes, escape sequences among them, straight at the terminal. GNU's says one thing
// instead, and since 3.5 says it on stderr so a pipe is not handed it:
//
//	grep: app.exe: binary file matches
//
// with status 0, as a match is. Git for Windows ships 3.0, which prints `Binary file app.exe
// matches` on stdout; the current form is the one followed. -c, -l, -L and -q answer for a
// binary file as for any other, since they print no lines of it. -a reads it as text, and -I
// as though it held no match.
//
// The test is a NUL in what the first read brought, which is up to 32K of a file and what has
// arrived of a pipe -- a pipe is not waited on for more, so `tail -f log | grep x` still shows a
// line when it comes -- and after that a NUL in a line, from which the file is binary. It is
// asked after decoding, so UTF-16 text, which is half NULs as it arrives, is text.

type grepBinaryMode int

const (
	grepBinaryDetect grepBinaryMode = iota
	// grepBinaryText is -a and --text: a binary file is read as text.
	grepBinaryText
	// grepBinarySkip is -I: a binary file is read as though it held no match.
	grepBinarySkip
)

// grepBinaryProbe is the most of a file the first read looks at.
const grepBinaryProbe = 32 * 1024

// probeBinary reports whether the input's first read holds a NUL, and hands back a reader that
// still has what was read.
func probeBinary(input io.Reader, mode grepBinaryMode) (io.Reader, bool) {
	if mode == grepBinaryText {
		return input, false
	}
	buffered := bufio.NewReaderSize(input, grepBinaryProbe)
	if _, err := buffered.Peek(1); err != nil {
		return buffered, false
	}
	first, _ := buffered.Peek(buffered.Buffered())
	return buffered, bytes.IndexByte(first, 0) >= 0
}

// binaryMatches says that a binary file matched, in GNU's words, on stderr.
func (p *grepPrinter) binaryMatches(name string) error {
	if name == "" {
		name = "(standard input)"
	}
	_, err := fmt.Fprintf(p.stderr, "grep: %s: binary file matches\n", name)
	return err
}
