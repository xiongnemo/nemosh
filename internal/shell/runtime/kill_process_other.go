//go:build !windows

package runtime

import (
	"fmt"
	"syscall"
)

// terminateProcess sends the signal, which off Windows is what kill means.
//
// A negative or zero pid addresses a process group, and that is deliberately
// passed through rather than refused: it is POSIX behaviour and the platform
// implements it, unlike Windows where there is nothing to pass it to.
func terminateProcess(pid, signal int) error {
	if err := syscall.Kill(pid, syscall.Signal(signal)); err != nil {
		return fmt.Errorf("%d: %w", pid, err)
	}
	return nil
}
