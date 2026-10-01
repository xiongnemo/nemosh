//go:build !windows

package main

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// keyWaiting reports whether a key arrives on the terminal within a wait, which tells vi
// mode's lone Escape from the start of a terminal sequence (lineedit_vi.go): busybox's
// read_key polls for it the same way.
func keyWaiting(file *os.File) func(time.Duration) bool {
	descriptor := int32(file.Fd())
	return func(wait time.Duration) bool {
		fds := []unix.PollFd{{Fd: descriptor, Events: unix.POLLIN}}
		for {
			n, err := unix.Poll(fds, int(wait.Milliseconds()))
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err == nil && n > 0
		}
	}
}
