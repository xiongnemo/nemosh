//go:build windows

package applets

import (
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// openShared opens a FILE tail reads, sharing delete as well as read and write: the program
// writing a log can rename it away while tail -F follows it, as it can on Linux. os.Open shares
// read and write alone, and busybox-w32's open as little, so `mv log log.1` under its tail -F
// was `Device or resource busy`.
func openShared(native string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: native, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		// A directory cannot be opened without backup semantics, and is refused as the other
		// applets refuse one.
		if info, statErr := os.Stat(native); statErr == nil && info.IsDir() {
			err = syscall.EISDIR
		}
		return nil, &fs.PathError{Op: "open", Path: native, Err: err}
	}
	return os.NewFile(uintptr(handle), native), nil
}
