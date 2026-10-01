//go:build !windows

package applets

import (
	"os"
	"syscall"
)

// directoryNameNotThere is what open(2) of a name with a slash after it fails with when
// O_CREAT asks for a file: EISDIR.
var directoryNameNotThere error = syscall.EISDIR

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

// renameForMove renames source to dest with rename(2), as busybox's mv does. os.Rename answers
// EEXIST for any directory at dest before the kernel is asked, so a file onto a directory was
// "File exists" where rename(2) and busybox say "Is a directory", and a directory onto an empty
// one was refused where rename(2) replaces it.
func renameForMove(source, dest string) error {
	if err := syscall.Rename(source, dest); err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: dest, Err: err}
	}
	return nil
}

// copyOwner gives the copy at native the owner and group info records.
func copyOwner(native string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Lchown(native, int(stat.Uid), int(stat.Gid))
}
