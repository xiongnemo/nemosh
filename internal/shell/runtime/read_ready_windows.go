//go:build windows

package runtime

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// procPeekNamedPipe is kernel32's PeekNamedPipe, which golang.org/x/sys/windows does not wrap.
var procPeekNamedPipe = windows.NewLazySystemDLL("kernel32.dll").NewProc("PeekNamedPipe")

// handleReady reports whether a read from handle would return at once: with data, or at the
// end of the input. A pipe is asked how much it holds, and one whose writer has gone is at
// its end; a file, and the null device, never wait; a console waits for a key.
func handleReady(handle uintptr) bool {
	fd := windows.Handle(handle)
	switch kind, _ := windows.GetFileType(fd); kind {
	case windows.FILE_TYPE_PIPE:
		var available uint32
		ok, _, _ := procPeekNamedPipe.Call(handle, 0, 0, 0, uintptr(unsafe.Pointer(&available)), 0)
		return ok == 0 || available > 0
	case windows.FILE_TYPE_CHAR:
		var mode uint32
		return windows.GetConsoleMode(fd, &mode) != nil
	}
	return true
}
