package oilsspec_test

import (
	"time"

	"golang.org/x/sys/windows"
)

// processEnds reports whether the process ends, or has ended, within the time given. One
// that has not is ended here, so a failing test leaves nothing behind.
func processEnds(pid int, within time.Duration) bool {
	process, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return true // gone, and its pid with it
	}
	defer windows.CloseHandle(process)
	event, err := windows.WaitForSingleObject(process, uint32(within.Milliseconds()))
	if err == nil && event == windows.WAIT_OBJECT_0 {
		return true
	}
	_ = windows.TerminateProcess(process, 1)
	return false
}
