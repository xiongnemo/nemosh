//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// Running the elevated shell in the window you are already in, rather than in
// one of its own.
//
// The obstacle is real: only the AppInfo service can create an elevated process,
// and it is reached through ShellExecuteEx or a COM elevation moniker, neither
// of which takes a STARTUPINFO. So the *launcher* cannot hand its console over.
//
// But the child can take it. AttachConsole attaches the calling process to the
// console of another process, and an elevated process attaching to the console
// of a medium-integrity one is allowed -- privilege runs the other way. So the
// handover happens on the far side: the shell is launched with no console of its
// own, and its first act is to attach to the console of the shell that launched
// it. This is what gsudo does, and it is why gsudo can run a command in place
// while ShellExecuteEx alone cannot.
//
// x/sys/windows does not wrap these two, so they are declared here.
var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	procFreeConsole   = kernel32.NewProc("FreeConsole")
)

// attachToConsoleOf joins the console of another process and returns the streams
// that now write to it.
//
// The handles have to be reopened. A process launched without a console started
// with no valid standard handles, so os.Stdout is not something that becomes
// useful once a console arrives -- CONIN$ and CONOUT$ are the names that resolve
// to the console currently attached, which is why they are opened after the
// attach and not before.
func attachToConsoleOf(pid int) (input, output *os.File, err error) {
	// Any console of our own is given up first. There should not be one, and if
	// there is, attaching would fail with ERROR_ACCESS_DENIED rather than
	// replacing it.
	_, _, _ = syscall.SyscallN(procFreeConsole.Addr())
	result, _, callErr := syscall.SyscallN(procAttachConsole.Addr(), uintptr(uint32(pid)))
	if result == 0 {
		return nil, nil, fmt.Errorf("cannot join the console of process %d: %w", pid, callErr)
	}
	if input, err = openConsoleStream("CONIN$", windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ); err != nil {
		return nil, nil, err
	}
	if output, err = openConsoleStream("CONOUT$", windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE); err != nil {
		input.Close()
		return nil, nil, err
	}
	return input, output, nil
}

// openConsoleStream opens one half of the attached console.
//
// GENERIC_READ|GENERIC_WRITE for both, including the input handle: the console
// mode calls the line editor makes -- raw mode, and the virtual terminal flag --
// need write access to a handle they also read.
func openConsoleStream(name string, access, share uint32) (*os.File, error) {
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr(name), access, share, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s on the joined console: %w", name, err)
	}
	return os.NewFile(uintptr(handle), name), nil
}

// hasConsole reports whether this process has a console another could join.
//
// By opening CONOUT$ rather than by asking GetConsoleWindow. The window handle
// answers a different question -- whether there is a *window* -- and under a
// pseudoconsole there is not one even though the console is entirely real and
// writable. Measured: this returns false through GetConsoleWindow in a session
// where CONOUT$ opens and reports a console mode.
//
// `su` asks before choosing how to launch: with no console there is nothing for
// the elevated shell to join, and a window of its own is the only place it can
// go.
func hasConsole() bool {
	output, err := openConsoleStream("CONOUT$", windows.GENERIC_WRITE, windows.FILE_SHARE_WRITE)
	if err != nil {
		return false
	}
	output.Close()
	return true
}
