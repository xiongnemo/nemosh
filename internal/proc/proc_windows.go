//go:build windows

package proc

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// stillActive is STILL_ACTIVE, what GetExitCodeProcess reports for a process that
// has not exited.
const stillActive = 259

// Terminate stops a process.
//
// Windows has no signals, so busybox does what this does: open the process and
// call TerminateProcess, with a comment saying plainly that it is not gentle --
// the target gets no chance to close handles or remove lock files
// (win32/process.c:900-910). Every signal therefore lands the same way, and that
// comment is the honest description of what `kill -TERM` means here.
//
// A pid that is not running is refused rather than reported as killed, which is
// the GetExitCodeProcess check busybox makes first. Telling a script that a
// signal reached a process which had already exited is the one answer worse than
// an error.
func Terminate(pid, signal int) error {
	if pid <= 0 {
		// Zero and negative address process groups, which Windows has not got in
		// the POSIX sense. Refusing beats guessing which processes were meant.
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

// List reports every process this session can see, excluding the caller.
//
// Excluding self is what busybox's pgrep does, and it is not tidiness: `pkill nemosh` that
// matched the shell running it would kill the thing being asked for the favour, and a pattern
// broad enough to match a shell is broad enough to be typed by accident.
//
// A projection of the system table now, rather than its own CreateToolhelp32Snapshot. That is
// what this package exists for -- one lookup, not one per caller -- and the table costs no more
// than the snapshot did while answering a great deal more. `top` does not come through here: it
// wants every process including this one, and it wants two samples to compare.
func List() ([]Process, error) {
	snapshot, err := NewSampler().Sample(false)
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	processes := make([]Process, 0, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		if process.PID == self || process.PID == 0 {
			continue
		}
		processes = append(processes, process)
	}
	return processes, nil
}
