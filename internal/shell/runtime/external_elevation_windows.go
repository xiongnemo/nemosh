//go:build windows

package runtime

import (
	"errors"
	"syscall"
)

// ERROR_ELEVATION_REQUIRED, 740. Written out rather than taken from
// golang.org/x/sys/windows so the number is visible next to the name it is
// being matched on; busybox-w32 defines it the same way and for the same reason
// (win32/process.c:7-9, where the toolchain may not have it either).
const errorElevationRequired = syscall.Errno(740)

// requiresElevation reports whether a launch failed only because the program
// asked for administrator. CreateProcess cannot elevate -- it is ShellExecuteEx
// with the `runas` verb or nothing -- so this is a whole class of program that
// is present, runnable, and unreachable from here.
func requiresElevation(err error) bool {
	return errors.Is(err, errorElevationRequired)
}

// elevationIsAWindowsIdea is what the shared test expects of the platform: only
// here does a launch fail for wanting administrator.
const elevationIsAWindowsIdea = true
