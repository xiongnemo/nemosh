//go:build !windows

package oilsspec_test

import (
	"errors"
	"syscall"
	"time"
)

// processEnds reports whether the process ends, or has ended, within the time given. One
// that has not is ended here, so a failing test leaves nothing behind.
func processEnds(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	return false
}
