//go:build linux

package applets

import "golang.org/x/sys/unix"

// The ioctls that read and write a termios on Linux. They are named differently on each
// system that has them, which is the only reason this is a file of its own.
const (
	termiosGet = unix.TCGETS
	termiosSet = unix.TCSETS
)
