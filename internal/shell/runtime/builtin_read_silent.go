package runtime

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/term"
)

// `read -s`: take the line without echoing it, because a password is being typed.
//
// This exists as its own path rather than as a flag threaded through the ordinary
// collector, because turning echo off is not a small change to reading a line --
// it also turns off the terminal's line discipline, so backspace stops working
// unless something handles it. x/term's ReadPassword is that something, and it is
// one call on both platforms: console mode on Windows, termios elsewhere.
//
// It reads a whole line, so it can only serve the spelling that wants one. The
// combinations it cannot serve are refused rather than served with the echo left
// on, because a password appearing on screen is the one outcome here that must not
// happen quietly.

// readSilently reports whether it handled the read.
//
// Not a terminal means there is no echo to suppress -- a pipe or a file shows
// nobody anything -- so `-s` falls through to the ordinary path, which is what
// bash does and what keeps `printf pw | read -s v` working in a test.
func (r Runtime) readSilently(ctx context.Context, input io.Reader, options readOptions) (readLineResult, bool, error) {
	if !options.silent {
		return readLineResult{}, false, nil
	}
	file, ok := terminalDescriptor(input)
	if !ok {
		return readLineResult{}, false, nil
	}
	if options.delimiter != '\n' || options.limit >= 0 {
		return readLineResult{}, true, fmt.Errorf(
			"-s cannot be combined with -d, -n or -N: echo is turned off by taking a whole line at a time")
	}
	if ctx.Err() != nil {
		return readLineResult{}, true, ctx.Err()
	}
	typed, err := term.ReadPassword(int(file))
	if err != nil {
		return readLineResult{}, true, err
	}
	// The newline the user pressed was consumed by ReadPassword and never drawn,
	// so the cursor is still on the prompt line. Ending it here is what stops the
	// next output from continuing the prompt.
	fmt.Fprintln(r.streams.Stderr)
	text, escaped := applyReadEscapes(string(typed), options.raw)
	return readLineResult{text: text, escaped: escaped, delimited: true}, true, nil
}

// terminalDescriptor answers with the descriptor behind a reader when it is a
// terminal.
//
// The fd table hands out an io.Reader, deliberately: nearly nothing in this shell
// should care what is behind it. This is one of the few places that must, and it
// asks by interface rather than by concrete type so a redirected `-u 3` pointing
// at the console works the same as stdin does.
func terminalDescriptor(input io.Reader) (uintptr, bool) {
	descriptor, ok := input.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}
	handle := descriptor.Fd()
	if !term.IsTerminal(int(handle)) {
		return 0, false
	}
	return handle, true
}

// applyReadEscapes processes backslashes over text already in hand, for the one
// caller that cannot read byte by byte.
//
// A trailing backslash cannot be a line continuation here: there is no second
// line to fetch, because the line discipline already ended this one. It is kept as
// data, which is the only remaining option and the same thing end-of-input does in
// the ordinary collector.
func applyReadEscapes(text string, raw bool) (string, []bool) {
	if raw {
		return text, make([]bool, len(text))
	}
	out := make([]byte, 0, len(text))
	escaped := make([]bool, 0, len(text))
	for index := 0; index < len(text); index++ {
		if text[index] == '\\' && index+1 < len(text) {
			index++
			out, escaped = append(out, text[index]), append(escaped, true)
			continue
		}
		out, escaped = append(out, text[index]), append(escaped, false)
	}
	return string(out), escaped
}
