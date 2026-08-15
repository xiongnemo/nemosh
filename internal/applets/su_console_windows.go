//go:build windows

package applets

import (
	"os"

	"golang.org/x/sys/windows"
)

// currentConsole answers for the console this process is in, which is what the
// elevated shell will be told to join.
func currentConsole() consoleHandover { return windowsConsole{} }

type windowsConsole struct{}

// usable asks by opening CONOUT$ rather than by looking for a window. Under a
// pseudoconsole -- which is what Windows Terminal gives every program it runs --
// there is no window, and the console is real and writable regardless.
func (windowsConsole) usable() bool {
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr("CONOUT$"),
		windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}

// ownerProcessID is this process. AttachConsole takes the pid of a process
// already attached to the console wanted, and this one is.
func (windowsConsole) ownerProcessID() int { return os.Getpid() }
