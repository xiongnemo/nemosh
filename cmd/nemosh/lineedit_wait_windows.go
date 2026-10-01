//go:build windows

package main

import (
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// kernel32's console input calls, which golang.org/x/sys/windows does not wrap.
var (
	procPeekConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekConsoleInputW")
	procReadConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")
)

// consoleInputRecord is INPUT_RECORD with its KEY_EVENT_RECORD, the one event read here.
type consoleInputRecord struct {
	eventType   uint16
	_           uint16
	keyDown     int32
	repeatCount uint16
	virtualKey  uint16
	scanCode    uint16
	char        uint16
	controlKeys uint32
}

// keyWaiting reports whether a key arrives on the console within a wait, which tells vi mode's
// lone Escape from the start of a terminal sequence (lineedit_vi.go). The handle is signalled
// for any input record, and a key coming up after the Escape went down is one, so the records
// a read would skip -- key-ups, focus, a modifier on its own -- are taken off first and the
// wait goes on.
func keyWaiting(file *os.File) func(time.Duration) bool {
	handle := windows.Handle(file.Fd())
	return func(wait time.Duration) bool {
		deadline := time.Now().Add(wait)
		for {
			left := max(time.Until(deadline), 0)
			if event, err := windows.WaitForSingleObject(handle, uint32(left.Milliseconds())); err != nil || event != windows.WAIT_OBJECT_0 {
				return false
			}
			var record consoleInputRecord
			var count uint32
			ok, _, _ := procPeekConsoleInput.Call(uintptr(handle), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)))
			if ok == 0 || count == 0 {
				return false
			}
			if record.eventType == windows.KEY_EVENT && record.keyDown != 0 && record.char != 0 {
				return true
			}
			procReadConsoleInput.Call(uintptr(handle), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)))
		}
	}
}
