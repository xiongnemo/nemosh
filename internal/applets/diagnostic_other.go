//go:build !windows

package applets

import (
	"errors"
	"syscall"
)

// syscall.Errno.Is folds ENOTEMPTY into fs.ErrExist here just as it folds
// ERROR_DIR_NOT_EMPTY into it on Windows, so the portable sentinels alone would
// spell a non-empty directory "File exists" on both. strerror calls it
// "Directory not empty", and a script comparing diagnostics across platforms
// expects to read the same words on each -- which is the whole point of this
// file having a Windows twin.
//
// Found by the first CI run on ubuntu-latest. Nothing had ever executed this
// package's tests on Linux before, so `rmdir` on a non-empty directory had been
// saying "File exists" there since it was written.
func platformCauseText(err error) (string, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return "", false
	}
	switch errno {
	case syscall.ENOTEMPTY:
		return "Directory not empty", true
	case syscall.ESPIPE:
		// Go spells it "illegal seek", which `dd seek=1 of=/dev/stdout | cat` says.
		return "Illegal seek", true
	}
	return "", false
}

func isCrossDeviceRename(err error) bool { return errors.Is(err, syscall.EXDEV) }
