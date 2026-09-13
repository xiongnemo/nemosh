package applets

import (
	"errors"
	"os"
	"syscall"
)

// Cancelling a read that is already blocked.
//
// readWithContext could only ever check cancellation *between* reads, so an applet sitting
// in a console read did not notice Ctrl-C until a keystroke arrived to end that read. At a
// terminal that is exactly what it looks like: `bc`, interrupted, does nothing until the
// next key, and the key that unblocks it is consumed doing so -- hence "I have to press
// Ctrl-C twice".
//
// Measured on a real console before writing this: a read blocked for 500ms, CancelIoEx
// answered success, and the read returned immediately. It reports the abort as EOF rather
// than ERROR_OPERATION_ABORTED, which is why the caller decides what a zero-length read
// means by asking the context rather than by reading the error.
var (
	readKernel32     = syscall.NewLazyDLL("kernel32.dll")
	readCancelIoEx   = readKernel32.NewProc("CancelIoEx")
	errReadNotFound  = syscall.Errno(1168) // ERROR_NOT_FOUND: nothing was pending
	errReadBadHandle = syscall.Errno(6)    // ERROR_INVALID_HANDLE: it has been closed
)

// interruptBlockedRead ends a read that is already waiting on this file.
//
// Nothing pending and a closed handle are both success: the read this was meant to end has
// already finished, which is the outcome asked for.
func interruptBlockedRead(file *os.File) {
	result, _, err := readCancelIoEx.Call(file.Fd(), 0)
	if result != 0 || errors.Is(err, errReadNotFound) || errors.Is(err, errReadBadHandle) {
		return
	}
}
