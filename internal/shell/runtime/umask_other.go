//go:build !windows

package runtime

import "syscall"

// initialFileModeMask is the umask the process was started with, as every shell's is: read by
// setting it and setting it back, once, while the package is initialised and nothing else is
// running to create a file in between.
var initialFileModeMask = processFileModeMask()

func processFileModeMask() uint16 {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return uint16(mask & 0o777)
}
