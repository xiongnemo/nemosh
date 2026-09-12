//go:build darwin

package applets

import "golang.org/x/sys/unix"

// The same two ioctls under the names the BSDs give them.
const (
	termiosGet = unix.TIOCGETA
	termiosSet = unix.TIOCSETA
)
