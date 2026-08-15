//go:build !windows

package main

import (
	"errors"
	"os"
)

// There is nothing to join here. A Unix terminal is a file descriptor, and a
// process that should write to it is given it -- which is exactly what Windows
// cannot do for an elevated child, and why the Windows half exists.
func attachToConsoleOf(int) (*os.File, *os.File, error) {
	return nil, nil, errors.New("joining another process's console is a Windows idea")
}

func hasConsole() bool { return false }
