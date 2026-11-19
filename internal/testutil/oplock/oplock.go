// Package oplock gives a test an open that waits, for as long as the test likes and on any
// NTFS volume, by holding a batch oplock on the file and never acknowledging its break. An
// open of that file waits as one to a share that has stopped answering does -- for the holder
// rather than for the network -- and it is ended the same way, by CancelSynchronousIo.
package oplock

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// FSCTL_REQUEST_BATCH_OPLOCK, CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 2, METHOD_BUFFERED,
// FILE_ANY_ACCESS).
const requestBatchOplock = 0x90008

// errNotGranted is ERROR_OPLOCK_NOT_GRANTED: another handle is open on the file, which on a
// runner is often the virus scanner, a moment after the file was written.
const errNotGranted = windows.Errno(300)

// Hold takes a batch oplock on the file at path and keeps it until the test ends, so that
// opening the file anywhere else waits meanwhile. The channel is closed when such an open
// arrives and the oplock starts to break, which is when that open starts waiting.
func Hold(t testing.TB, path string) <-chan struct{} {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("oplock path: %v", err)
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		t.Fatalf("oplock event: %v", err)
	}
	overlapped := &windows.Overlapped{HEvent: event}
	var holder windows.Handle
	for deadline := time.Now().Add(5 * time.Second); ; {
		holder, err = request(name, overlapped)
		if !errors.Is(err, errNotGranted) || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		windows.CloseHandle(event)
		t.Fatalf("batch oplock on %s: %v", path, err)
	}
	breaking := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		windows.WaitForSingleObject(event, windows.INFINITE)
		close(breaking)
	}()
	t.Cleanup(func() {
		// Closing the holder lets the waiting open go on, and completes the request, which
		// sets the event; set here as well, for a request that had already completed.
		windows.CloseHandle(holder)
		windows.SetEvent(event)
		<-watched
		windows.CloseHandle(event)
		// The request wrote into it until the holder closed.
		runtime.KeepAlive(overlapped)
	})
	return breaking
}

// request opens the file for the oplock alone and asks for it; the request stays pending
// while the oplock is held.
func request(name *uint16, overlapped *windows.Overlapped) (windows.Handle, error) {
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	holder, err := windows.CreateFile(name, windows.GENERIC_READ, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return 0, err
	}
	err = windows.DeviceIoControl(holder, requestBatchOplock, nil, 0, nil, 0, nil, overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		return holder, nil
	}
	windows.CloseHandle(holder)
	if err == nil {
		err = errors.New("the request completed at once, so no oplock is held")
	}
	return 0, err
}
