//go:build windows

package proc

import (
	"fmt"
	"os"
	"unsafe"

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
// Excluding self is what busybox's pgrep does, and it is not tidiness: `pkill
// nemosh` that matched the shell running it would kill the thing being asked for
// the favour, and a pattern broad enough to match a shell is broad enough to be
// typed by accident.
func List() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	self := os.Getpid()
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var processes []Process
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		pid := int(entry.ProcessID)
		if pid == self || pid == 0 {
			continue
		}
		processes = append(processes, Process{PID: pid, Name: windows.UTF16ToString(entry.ExeFile[:])})
	}
	if err != nil && err != windows.ERROR_NO_MORE_FILES {
		return nil, fmt.Errorf("listing processes: %w", err)
	}
	return processes, nil
}
