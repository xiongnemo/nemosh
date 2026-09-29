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

// removeDirectory removes an empty directory.
func removeDirectory(native string) error { return os.Remove(native) }

// canWrite is whether this process may write the file, as access(2) answers.
func canWrite(native string, _ os.FileInfo) bool { return syscall.Access(native, 2) == nil }

// renameForMove renames source to dest.
func renameForMove(source, dest string) error { return os.Rename(source, dest) }

// copyOwner gives the copy at native the owner and group info records.
func copyOwner(native string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Lchown(native, int(stat.Uid), int(stat.Gid))
}
