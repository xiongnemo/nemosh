//go:build windows

package runtime

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// terminateProcess kills a real process by pid.
//
// Windows has no signals, so busybox does the same thing this does: open the
// process and call TerminateProcess (win32/process.c:909), with a comment saying
// plainly that it is not gentle -- the target gets no chance to close handles or
// remove lock files. Every signal therefore lands the same way here, and the
// wording of the comment there is the honest description of what `kill -TERM`
// means on this platform.
//
// A pid that is not running is refused rather than reported as killed, which is
// the check busybox makes with GetExitCodeProcess before it terminates anything:
// telling a script that a signal reached a process that had already exited is
// the one answer worse than an error.
func terminateProcess(pid, signal int) error {
	if pid <= 0 {
		// Zero and negative mean process groups, which Windows does not have in
		// the POSIX sense. Refusing is better than guessing which processes were
		// meant.
		return fmt.Errorf("%d: this build signals a single process id", pid)
	}
	const access = windows.PROCESS_TERMINATE | windows.PROCESS_QUERY_LIMITED_INFORMATION
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("%d: %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return fmt.Errorf("%d: %w", pid, err)
	}
	if code != stillActive {
		return fmt.Errorf("%d: no such process", pid)
	}
	// The exit code busybox leaves behind, so a wrapper reading it sees the same
	// number under either shell.
	if err := windows.TerminateProcess(handle, uint32(signal)<<24); err != nil {
		return fmt.Errorf("%d: %w", pid, err)
	}
	return nil
}

// stillActive is STILL_ACTIVE, which is what GetExitCodeProcess reports for a
// process that has not exited.
const stillActive = 259
