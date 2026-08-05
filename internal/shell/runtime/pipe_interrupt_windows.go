package runtime

import (
	"errors"
	"os"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	cancelIoEx       = kernel32.NewProc("CancelIoEx")
	errNotFound      = syscall.Errno(1168)
	errInvalidHandle = syscall.Errno(6)
)

func interruptPipeIO(file *os.File) error {
	result, _, callErr := cancelIoEx.Call(file.Fd(), 0)
	if result != 0 || errors.Is(callErr, errNotFound) || errors.Is(callErr, errInvalidHandle) {
		return nil
	}
	return callErr
}
