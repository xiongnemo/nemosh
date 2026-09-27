//go:build !windows

package runtime

import "golang.org/x/sys/unix"

// handleReady reports whether a read from handle would return at once: with data, or at the
// end of the input, which poll answers for every kind of descriptor.
func handleReady(handle uintptr) bool {
	fds := []unix.PollFd{{Fd: int32(handle), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0)
	return err != nil || n > 0
}
