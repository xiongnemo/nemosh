package applets

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

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

// copyOwner has nothing to do: a Windows file's owner is its creator, and busybox-w32's chown
// changes nothing either.
func copyOwner(string, os.FileInfo) error { return nil }
