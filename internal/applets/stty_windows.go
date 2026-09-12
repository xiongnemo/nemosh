package applets

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Reading and setting the console's echo bit on Windows.
//
// A Windows console has a **mode word** rather than a termios structure, and echo is one bit
// of it: `ENABLE_ECHO_INPUT`. The bit only means anything while `ENABLE_LINE_INPUT` is also
// set -- the console echoes as part of assembling a line -- so turning echo off means
// clearing both, which is also what makes the input arrive a key at a time rather than a
// line at a time. That is exactly what a password prompt wants, and it is why `stty -echo`
// and a raw-mode toggle are nearly the same operation here where on Unix they are not.

// terminalEcho reports whether the console is echoing what is typed.
func terminalEcho(handle *os.File) (bool, error) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(handle.Fd()), &mode); err != nil {
		return false, fmt.Errorf("cannot read the console mode: %s", CauseText(err))
	}
	return mode&windows.ENABLE_ECHO_INPUT != 0, nil
}

func setTerminalEcho(handle *os.File, on bool) error {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(handle.Fd()), &mode); err != nil {
		return fmt.Errorf("cannot read the console mode: %s", CauseText(err))
	}
	if on {
		mode |= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT
	} else {
		// Both bits, because the console will not stop echoing while it is still
		// assembling lines for us.
		mode &^= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT
	}
	if err := windows.SetConsoleMode(windows.Handle(handle.Fd()), mode); err != nil {
		return fmt.Errorf("cannot set the console mode: %s", CauseText(err))
	}
	return nil
}

// makeTerminalSane puts back the three bits a shell expects to find.
//
// Not the whole mode word: something else may have turned on virtual-terminal processing or
// mouse input deliberately, and `stty sane` is asked for when the *typing* has stopped
// working, not to undo everything.
func makeTerminalSane(handle *os.File) error {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(handle.Fd()), &mode); err != nil {
		return fmt.Errorf("cannot read the console mode: %s", CauseText(err))
	}
	mode |= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
	if err := windows.SetConsoleMode(windows.Handle(handle.Fd()), mode); err != nil {
		return fmt.Errorf("cannot set the console mode: %s", CauseText(err))
	}
	return nil
}
