package applets

import (
	"errors"
	"syscall"
)

// EXDEV's Win32 spelling. Go's syscall package does not export it, and
// MoveFileEx reports it rather than the synthetic syscall.EXDEV.
const errorNotSameDevice = syscall.Errno(17)

// syscall.Errno.Is folds ERROR_DIR_NOT_EMPTY into fs.ErrExist, so the portable
// sentinels alone would spell a non-empty directory "File exists". POSIX has a
// distinct ENOTEMPTY and strerror calls it "Directory not empty", which is what
// a script comparing diagnostics across platforms expects to read.
func platformCauseText(err error) (string, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return "", false
	}
	if errno == syscall.ERROR_DIR_NOT_EMPTY {
		return "Directory not empty", true
	}
	return "", false
}

func isCrossDeviceRename(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorNotSameDevice || errno == syscall.EXDEV
}
