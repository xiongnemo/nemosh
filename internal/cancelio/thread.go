package cancelio

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// kernel32's two calls x/sys has no binding for, looked up on first use rather than at start:
// a lazy proc made at package level is an allocation in every process's init.
var kernel32 struct {
	once         sync.Once
	cancel       *windows.LazyProc
	threadCycles *windows.LazyProc
}

func procs() {
	kernel32.once.Do(func() {
		dll := windows.NewLazySystemDLL("kernel32.dll")
		kernel32.cancel = dll.NewProc("CancelSynchronousIo")
		kernel32.threadCycles = dll.NewProc("QueryThreadCycleTime")
	})
}

// Cancel is CancelSynchronousIo: it ends the synchronous call thread is waiting in, which then
// fails with ERROR_OPERATION_ABORTED. ERROR_NOT_FOUND is a thread waiting in none.
func Cancel(thread windows.Handle) error {
	procs()
	if result, _, err := kernel32.cancel.Call(uintptr(thread)); result == 0 {
		return err
	}
	return nil
}

// cycleTime is QueryThreadCycleTime: the processor cycles the thread has run, in user mode and
// in the kernel. A thread waiting runs none.
func cycleTime(thread windows.Handle) (uint64, bool) {
	procs()
	var cycles uint64
	result, _, _ := kernel32.threadCycles.Call(uintptr(thread), uintptr(unsafe.Pointer(&cycles)))
	return cycles, result != 0
}
