package applets

import (
	"errors"
	"io/fs"
	"syscall"
)

// EXDEV's Win32 spelling. Go's syscall package does not export it, and
// MoveFileEx reports it rather than the synthetic syscall.EXDEV.
const errorNotSameDevice = syscall.Errno(17)

// platformCauseText words a Windows error as busybox-w32 words it: the text of the errno its
// err_win_to_posix makes of it (diagnostic_windows_errno.go). A file in use is `Permission
// denied`, where Windows says the process cannot access the file because another process is
// using it. Asking about a file follows get_file_attr instead (win32/mingw.c:448-462), which
// calls most failures `No such file or directory`: a name Windows cannot hold, `ls 'x*'`, is not
// there, where opening one is an invalid argument, as in busybox.
//
// syscall.Errno.Is folds ERROR_DIR_NOT_EMPTY into fs.ErrExist, so the portable sentinels alone
// would spell a non-empty directory "File exists". POSIX has a distinct ENOTEMPTY and strerror
// calls it "Directory not empty", which the table says too.
func platformCauseText(err error) (string, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return "", false
	}
	if isStatOperation(err) {
		switch errno {
		case syscall.ERROR_ACCESS_DENIED, 32, 33, 36: // sharing and lock violations
			return eacces, true
		case 111: // ERROR_BUFFER_OVERFLOW
			return enametoolong, true
		case 8: // ERROR_NOT_ENOUGH_MEMORY
			return enomem, true
		}
		return enoent, true
	}
	text, ok := windowsErrnoText[errno]
	return text, ok
}

// isStatOperation is whether err came from asking about a file, the calls os.Stat and os.Lstat
// make, rather than from opening or changing one.
func isStatOperation(err error) bool {
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	switch pathErr.Op {
	case "Stat", "Lstat", "GetFileAttributesEx", "FindFirstFile", "CreateFile", "GetFileType":
		return true
	}
	return false
}

func isCrossDeviceRename(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorNotSameDevice || errno == syscall.EXDEV
}
