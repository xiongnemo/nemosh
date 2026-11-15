//go:build !windows

package runtime

import "os"

// parentProcessID is the process that started this one, for $PPID: getppid(2), which
// is a system call here rather than the walk it is on Windows.
func parentProcessID() int {
	return os.Getppid()
}
