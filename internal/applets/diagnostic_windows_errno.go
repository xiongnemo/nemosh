package applets

import "syscall"

// The errno texts busybox-w32 prints, as the C runtime's strerror words them (msvcrt), and
// ELOOP's, which mingw_strerror supplies (win32/mingw.c:160).
const (
	e2big        = "Arg list too long"
	eacces       = "Permission denied"
	eagain       = "Resource temporarily unavailable"
	ebadf        = "Bad file descriptor"
	ebusy        = "Resource device"
	echild       = "No child processes"
	eexist       = "File exists"
	efault       = "Bad address"
	eintr        = "Interrupted function call"
	einval       = "Invalid argument"
	eio          = "Input/output error"
	eloop        = "Too many levels of symbolic links"
	emfile       = "Too many open files"
	emlink       = "Too many links"
	enametoolong = "Filename too long"
	enfile       = "Too many open files in system"
	enodev       = "No such device"
	enoent       = "No such file or directory"
	enoexec      = "Exec format error"
	enomem       = "Not enough space"
	enospc       = "No space left on device"
	enosys       = "Function not implemented"
	enotempty    = "Directory not empty"
	enxio        = "No such device or address"
	eperm        = "Operation not permitted"
	epipe        = "Broken pipe"
	erange       = "Result too large"
	erofs        = "Read-only file system"
	espipe       = "Invalid seek"
	exdev        = "Improper link"
)

// windowsErrnoText is busybox-w32's err_win_to_posix (win32/mingw.c:51-157): each Windows error
// by the text of the errno it becomes there, so a message says `Permission denied` where
// Windows would say that the process cannot access the file because another is using it.
func windowsErrnoText(key syscall.Errno) (string, bool) {
	switch key {
	case 1, 120:
		return enosys, true
	case 2, 3, 53, 67, 161, 999:
		return enoent, true
	case 4, 214:
		return emfile, true
	case 5, 12, 16, 32, 33, 82, 1313, 1314, 1326, 1327, 1328, 1329, 1330, 1331:
		return eacces, true
	case 6:
		return ebadf, true
	case 8, 14, 122, 1001:
		return enomem, true
	case 9, 487, 998:
		return efault, true
	case 11, 191, 192, 193:
		return enoexec, true
	case 13, 24, 87, 123, 203, 209, 267, 1004, 1132, 1307, 1308, 1332, 2202:
		return einval, true
	case 15, 20, 55, 1005, 1006, 1200:
		return enodev, true
	case 17:
		return exdev, true
	case 19:
		return erofs, true
	case 21, 89:
		return eagain, true
	case 22, 23, 25, 29, 30, 31, 107, 110, 113, 114, 1011, 1012, 1013, 1117:
		return eio, true
	case 36:
		return enfile, true
	case 39, 112:
		return enospc, true
	case 80, 183:
		return eexist, true
	case 85, 108, 142, 148, 170, 212, 231, 2401, 2404:
		return ebusy, true
	case 86:
		return eperm, true
	case 109, 230, 232, 233, 234, 535, 536:
		return epipe, true
	case 111, 206:
		return enametoolong, true
	case 119, 1785:
		return enxio, true
	case 128:
		return echild, true
	case 131, 132:
		return espipe, true
	case 145:
		return enotempty, true
	case 208:
		return e2big, true
	case 534:
		return erange, true
	case 995, 996:
		return eintr, true
	case 1142:
		return emlink, true
	case 1921:
		return eloop, true
	}
	var zero string
	return zero, false
}
