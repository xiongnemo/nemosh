//go:build !windows

package applets

import (
	"os"
	"syscall"
)

// removeForOverwrite removes a file in a copy's way, as unlink would: a directory is refused,
// where os.Remove would take an empty one.
func removeForOverwrite(native string) error {
	if info, err := os.Lstat(native); err == nil && info.IsDir() {
		return syscall.EISDIR
	}
	return os.Remove(native)
}

// copyOwner gives the copy at native the owner and group info records.
func copyOwner(native string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Lchown(native, int(stat.Uid), int(stat.Gid))
}
