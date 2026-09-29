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
var windowsErrnoText = map[syscall.Errno]string{
	1:    enosys,       // ERROR_INVALID_FUNCTION
	2:    enoent,       // ERROR_FILE_NOT_FOUND
	3:    enoent,       // ERROR_PATH_NOT_FOUND
	4:    emfile,       // ERROR_TOO_MANY_OPEN_FILES
	5:    eacces,       // ERROR_ACCESS_DENIED
	6:    ebadf,        // ERROR_INVALID_HANDLE
	8:    enomem,       // ERROR_NOT_ENOUGH_MEMORY
	9:    efault,       // ERROR_INVALID_BLOCK
	11:   enoexec,      // ERROR_BAD_FORMAT
	12:   eacces,       // ERROR_INVALID_ACCESS
	13:   einval,       // ERROR_INVALID_DATA
	14:   enomem,       // ERROR_OUTOFMEMORY
	15:   enodev,       // ERROR_INVALID_DRIVE
	16:   eacces,       // ERROR_CURRENT_DIRECTORY
	17:   exdev,        // ERROR_NOT_SAME_DEVICE
	19:   erofs,        // ERROR_WRITE_PROTECT
	20:   enodev,       // ERROR_BAD_UNIT
	21:   eagain,       // ERROR_NOT_READY
	22:   eio,          // ERROR_BAD_COMMAND
	23:   eio,          // ERROR_CRC
	24:   einval,       // ERROR_BAD_LENGTH
	25:   eio,          // ERROR_SEEK
	29:   eio,          // ERROR_WRITE_FAULT
	30:   eio,          // ERROR_READ_FAULT
	31:   eio,          // ERROR_GEN_FAILURE
	32:   eacces,       // ERROR_SHARING_VIOLATION
	33:   eacces,       // ERROR_LOCK_VIOLATION
	36:   enfile,       // ERROR_SHARING_BUFFER_EXCEEDED
	39:   enospc,       // ERROR_HANDLE_DISK_FULL
	53:   enoent,       // ERROR_BAD_NETPATH
	55:   enodev,       // ERROR_DEV_NOT_EXIST
	67:   enoent,       // ERROR_BAD_NET_NAME
	80:   eexist,       // ERROR_FILE_EXISTS
	82:   eacces,       // ERROR_CANNOT_MAKE
	85:   ebusy,        // ERROR_ALREADY_ASSIGNED
	86:   eperm,        // ERROR_INVALID_PASSWORD
	87:   einval,       // ERROR_INVALID_PARAMETER
	89:   eagain,       // ERROR_NO_PROC_SLOTS
	107:  eio,          // ERROR_DISK_CHANGE
	108:  ebusy,        // ERROR_DRIVE_LOCKED
	109:  epipe,        // ERROR_BROKEN_PIPE
	110:  eio,          // ERROR_OPEN_FAILED
	111:  enametoolong, // ERROR_BUFFER_OVERFLOW
	112:  enospc,       // ERROR_DISK_FULL
	113:  eio,          // ERROR_NO_MORE_SEARCH_HANDLES
	114:  eio,          // ERROR_INVALID_TARGET_HANDLE
	119:  enxio,        // ERROR_BAD_DRIVER_LEVEL
	120:  enosys,       // ERROR_CALL_NOT_IMPLEMENTED
	122:  enomem,       // ERROR_INSUFFICIENT_BUFFER
	123:  einval,       // ERROR_INVALID_NAME
	128:  echild,       // ERROR_WAIT_NO_CHILDREN
	131:  espipe,       // ERROR_NEGATIVE_SEEK
	132:  espipe,       // ERROR_SEEK_ON_DEVICE
	142:  ebusy,        // ERROR_BUSY_DRIVE
	145:  enotempty,    // ERROR_DIR_NOT_EMPTY
	148:  ebusy,        // ERROR_PATH_BUSY
	161:  enoent,       // ERROR_BAD_PATHNAME
	170:  ebusy,        // ERROR_BUSY
	183:  eexist,       // ERROR_ALREADY_EXISTS
	191:  enoexec,      // ERROR_INVALID_EXE_SIGNATURE
	192:  enoexec,      // ERROR_EXE_MARKED_INVALID
	193:  enoexec,      // ERROR_BAD_EXE_FORMAT
	203:  einval,       // ERROR_ENVVAR_NOT_FOUND
	206:  enametoolong, // ERROR_FILENAME_EXCED_RANGE
	208:  e2big,        // ERROR_META_EXPANSION_TOO_LONG
	209:  einval,       // ERROR_INVALID_SIGNAL_NUMBER
	212:  ebusy,        // ERROR_LOCKED
	214:  emfile,       // ERROR_TOO_MANY_MODULES
	230:  epipe,        // ERROR_BAD_PIPE
	231:  ebusy,        // ERROR_PIPE_BUSY
	232:  epipe,        // ERROR_NO_DATA
	233:  epipe,        // ERROR_PIPE_NOT_CONNECTED
	234:  epipe,        // ERROR_MORE_DATA
	267:  einval,       // ERROR_DIRECTORY
	487:  efault,       // ERROR_INVALID_ADDRESS
	534:  erange,       // ERROR_ARITHMETIC_OVERFLOW
	535:  epipe,        // ERROR_PIPE_CONNECTED
	536:  epipe,        // ERROR_PIPE_LISTENING
	995:  eintr,        // ERROR_OPERATION_ABORTED
	996:  eintr,        // ERROR_IO_INCOMPLETE
	998:  efault,       // ERROR_NOACCESS
	999:  enoent,       // ERROR_SWAPERROR
	1001: enomem,       // ERROR_STACK_OVERFLOW
	1004: einval,       // ERROR_INVALID_FLAGS
	1005: enodev,       // ERROR_UNRECOGNIZED_VOLUME
	1006: enodev,       // ERROR_FILE_INVALID
	1011: eio,          // ERROR_CANTOPEN
	1012: eio,          // ERROR_CANTREAD
	1013: eio,          // ERROR_CANTWRITE
	1117: eio,          // ERROR_IO_DEVICE
	1132: einval,       // ERROR_MAPPED_ALIGNMENT
	1142: emlink,       // ERROR_TOO_MANY_LINKS
	1200: enodev,       // ERROR_BAD_DEVICE
	1307: einval,       // ERROR_INVALID_OWNER
	1308: einval,       // ERROR_INVALID_PRIMARY_GROUP
	1313: eacces,       // ERROR_NO_SUCH_PRIVILEGE
	1314: eacces,       // ERROR_PRIVILEGE_NOT_HELD
	1326: eacces,       // ERROR_LOGON_FAILURE
	1327: eacces,       // ERROR_ACCOUNT_RESTRICTION
	1328: eacces,       // ERROR_INVALID_LOGON_HOURS
	1329: eacces,       // ERROR_INVALID_WORKSTATION
	1330: eacces,       // ERROR_PASSWORD_EXPIRED
	1331: eacces,       // ERROR_ACCOUNT_DISABLED
	1332: einval,       // ERROR_NONE_MAPPED
	1785: enxio,        // ERROR_UNRECOGNIZED_MEDIA
	1921: eloop,        // ERROR_CANT_RESOLVE_FILENAME
	2202: einval,       // ERROR_BAD_USERNAME
	2401: ebusy,        // ERROR_OPEN_FILES
	2404: ebusy,        // ERROR_DEVICE_IN_USE
}
