package runtime

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hostConsoleTitle is the title of this process's console, through SetConsoleTitleW and
// GetConsoleTitleW, which busybox-w32's winansi.c calls. The procedures are found when title
// is run, not as the binary starts.
type hostConsoleTitle struct{}

func (hostConsoleTitle) setTitle(text string) error {
	name, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return err
	}
	if result, _, callErr := kernel32.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(name))); result == 0 {
		return callErr
	}
	return nil
}

// title reads into a buffer of the longest title a console holds. A length of 0 is an empty
// title unless Windows says it was an error.
func (hostConsoleTitle) title() (string, error) {
	buffer := make([]uint16, 32767)
	length, _, callErr := kernel32.NewProc("GetConsoleTitleW").Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 && callErr != syscall.Errno(0) {
		return "", callErr
	}
	return windows.UTF16ToString(buffer[:length]), nil
}
