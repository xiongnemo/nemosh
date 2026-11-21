package runtime

import (
	"errors"
	"io"
	"os"
)

// /dev/tty is the console this shell is attached to, as a terminal is on Unix: CONIN$ to read
// and CONOUT$ to write. Each is opened as the console's own handle, an *os.File, which a
// program the shell starts is handed as it is, so `fzf </dev/tty` or zoxide's interactive pick
// has the terminal rather than a pipe. busybox-w32 has no /dev/tty; Git's bash has one, and a
// script written for it reaches for it.

// openConsoleFile opens one of the console's two names; a variable so a test can stand a file
// in for each, whether or not it has a console.
var openConsoleFile = func(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR, 0)
}

// errNoTerminal is /dev/tty with no console behind it -- a service, a CI runner -- in the
// words of Git's bash, which a script written for it would know.
var errNoTerminal = errors.New("No such device or address")

func openTerminal(name string) (*os.File, error) {
	file, err := openConsoleFile(name)
	if err != nil {
		return nil, errNoTerminal
	}
	return file, nil
}

func openTerminalReader() (io.ReadCloser, error) { return openTerminal("CONIN$") }

func openTerminalWriter(bool) (io.WriteCloser, error) { return openTerminal("CONOUT$") }
