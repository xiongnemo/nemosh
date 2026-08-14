//go:build !windows

package proc

import (
	"fmt"
	"syscall"
)

// Terminate sends the signal, which off Windows is what kill means.
//
// A negative or zero pid addresses a process group, and that is passed through
// rather than refused: it is POSIX behaviour and the platform implements it,
// unlike Windows where there is nothing to pass it to.
func Terminate(pid, signal int) error {
	if err := syscall.Kill(pid, syscall.Signal(signal)); err != nil {
		return fmt.Errorf("%d: %w", pid, err)
	}
	return nil
}

// List is refused here rather than approximated.
//
// Reading /proc would work on Linux and nothing portable works on macOS, and both
// are build-and-test targets rather than supported ones. A capability that is
// absent must fail loudly, so this says so instead of returning an empty list
// that would make `pgrep anything` answer "no match" on a busy machine.
func List() ([]Process, error) { return nil, ErrListUnsupported }
