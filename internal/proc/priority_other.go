//go:build !windows

package proc

import "fmt"

// AdjustPriority refuses off Windows, as the rest of this package does.
//
// Not implemented with setpriority: this package's whole subject is the Windows process table, and
// a half-package that changed priorities on Linux while being unable to list a process there would
// be worse than one that says where it works.
func AdjustPriority(pid, step int) (string, error) {
	return "", fmt.Errorf("%d: changing priority is implemented on Windows only", pid)
}
