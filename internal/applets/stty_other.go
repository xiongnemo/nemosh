//go:build linux || darwin

package applets

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// The same three questions, asked of a termios line discipline.
//
// Echo here is one flag in `c_lflag` and nothing else has to move with it, which is the
// difference from Windows: a Unix terminal can stop echoing and still assemble lines, so
// `stty -echo` leaves canonical mode alone.
//
// This build exists so the package compiles and its tests run on the CI runners. Those
// systems already ship a complete `stty`, and this one answers only the handful of questions
// the Windows console can -- which is what keeps the two spellings of the command honest
// about meaning the same thing.

func terminalEcho(handle *os.File) (bool, error) {
	settings, err := unix.IoctlGetTermios(int(handle.Fd()), termiosGet)
	if err != nil {
		return false, fmt.Errorf("cannot read the terminal settings: %s", CauseText(err))
	}
	return settings.Lflag&unix.ECHO != 0, nil
}

func setTerminalEcho(handle *os.File, on bool) error {
	settings, err := unix.IoctlGetTermios(int(handle.Fd()), termiosGet)
	if err != nil {
		return fmt.Errorf("cannot read the terminal settings: %s", CauseText(err))
	}
	if on {
		settings.Lflag |= unix.ECHO
	} else {
		settings.Lflag &^= unix.ECHO
	}
	if err := unix.IoctlSetTermios(int(handle.Fd()), termiosSet, settings); err != nil {
		return fmt.Errorf("cannot set the terminal settings: %s", CauseText(err))
	}
	return nil
}

// makeTerminalSane restores the flags a shell expects, and only those.
func makeTerminalSane(handle *os.File) error {
	settings, err := unix.IoctlGetTermios(int(handle.Fd()), termiosGet)
	if err != nil {
		return fmt.Errorf("cannot read the terminal settings: %s", CauseText(err))
	}
	settings.Lflag |= unix.ECHO | unix.ICANON | unix.ISIG
	if err := unix.IoctlSetTermios(int(handle.Fd()), termiosSet, settings); err != nil {
		return fmt.Errorf("cannot set the terminal settings: %s", CauseText(err))
	}
	return nil
}
