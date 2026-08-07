//go:build windows

package runtime

import (
	"syscall"
	"time"
)

// processCPUTime asks Windows for this process's own CPU use. GetProcessTimes
// reports four FILETIMEs and only the last two are CPU; the first two are the
// wall-clock instants the process started and exited.
func processCPUTime() (time.Duration, time.Duration, error) {
	var creation, exit, kernel, user syscall.Filetime
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, 0, err
	}
	if err := syscall.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, 0, err
	}
	return filetimeDuration(user), filetimeDuration(kernel), nil
}

// A FILETIME counts 100-nanosecond intervals.
func filetimeDuration(t syscall.Filetime) time.Duration {
	return time.Duration(t.Nanoseconds())
}
