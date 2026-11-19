package cancelio

import (
	"sync"

	"golang.org/x/sys/windows"
)

// CancelSynchronousIo, which x/sys has no binding for, looked up on first use rather than at
// start: a lazy proc made at package level is an allocation in every process's init.
var kernel32 struct {
	once   sync.Once
	cancel *windows.LazyProc
}

// Cancel is CancelSynchronousIo: it ends the synchronous call thread is waiting in, which then
// fails with ERROR_OPERATION_ABORTED. ERROR_NOT_FOUND is a thread waiting in none.
func Cancel(thread windows.Handle) error {
	kernel32.once.Do(func() {
		kernel32.cancel = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")
	})
	if result, _, err := kernel32.cancel.Call(uintptr(thread)); result == 0 {
		return err
	}
	return nil
}
