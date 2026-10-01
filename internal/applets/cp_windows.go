package applets

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// directoryNameNotThere is what creating a file named with a separator after it fails with:
// ERROR_INVALID_NAME, which busybox-w32 makes EINVAL, `Invalid argument`.
var directoryNameNotThere error = syscall.Errno(123)

// removeForOverwrite removes a file in a copy's way, as unlink would: a directory is refused,
// where os.Remove would take an empty one. A read-only file is made writable first, as
// busybox-w32's unlink does (win32/mingw.c:1954), since Windows will not delete it otherwise.
func removeForOverwrite(native string) error {
	if info, err := os.Lstat(native); err == nil && info.IsDir() {
		return syscall.EISDIR
	}
	err := os.Remove(native)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	if chmodErr := os.Chmod(native, 0o666); chmodErr != nil {
		return err
	}
	return os.Remove(native)
}

// removeDirectory removes an empty directory. A read-only one is made writable first, as
// busybox-w32's rmdir does (win32/mingw.c:2152).
func removeDirectory(native string) error {
	err := os.Remove(native)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	if chmodErr := os.Chmod(native, 0o777); chmodErr != nil {
		return err
	}
	return os.Remove(native)
}

// canWrite is whether a file may be written, which on Windows is whether it is not read-only.
func canWrite(_ string, info os.FileInfo) bool { return info.Mode().Perm()&0o200 != 0 }

// renameForMove renames source to dest as busybox-w32's rename does (win32/mingw.c:1168): a
// read-only destination is made writable and replaced, and a directory in the way is `Is a
// directory` rather than access denied.
func renameForMove(source, dest string) error {
	err := os.Rename(source, dest)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	info, statErr := os.Stat(dest)
	if statErr != nil {
		return err
	}
	if info.IsDir() {
		return &os.LinkError{Op: "rename", Old: source, New: dest, Err: syscall.EISDIR}
	}
	if info.Mode().Perm()&0o200 != 0 || os.Chmod(dest, 0o666) != nil {
		return err
	}
	if retry := os.Rename(source, dest); retry != nil {
		_ = os.Chmod(dest, info.Mode().Perm())
		return retry
	}
	return nil
}

// copyOwner has nothing to do: a Windows file's owner is its creator, and busybox-w32's chown
// changes nothing either.
func copyOwner(string, os.FileInfo) error { return nil }
