// Package stall gives a test a call that waits until something cancels it: a read of a named
// pipe whose server accepts the reader and never writes, opened by path as a file on a share is.
// CancelSynchronousIo ends that read on every Windows the tests run on.
//
// An oplock was the first such wait -- an open of a file whose batch oplock's holder never
// answers the break -- and it serves on Windows 10, but on the runners' Windows an open waiting
// on an oplock can be cancelled only as it begins: a cancel 60 ms into the wait itself waited,
// ten minutes, until the test was killed (2026-11-19).
package stall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/windows"
)

var made atomic.Uint64

// Pipe makes a named pipe nothing is ever written to, and answers its path in the shell's
// spelling, `//./pipe/...`, and a channel that is closed when a reader has opened it. The pipe
// goes when the test ends, which ends the read.
func Pipe(t testing.TB) (string, <-chan struct{}) {
	t.Helper()
	name := fmt.Sprintf(`\\.\pipe\nemosh-stall-%d-%d`, os.Getpid(), made.Add(1))
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	server, err := windows.CreateNamedPipe(path, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT, 1, 4096, 4096, 0, nil)
	if err != nil {
		t.Fatalf("named pipe: %v", err)
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(server)
		t.Fatalf("named pipe event: %v", err)
	}
	overlapped := &windows.Overlapped{HEvent: event}
	opened := make(chan struct{})
	watched := make(chan struct{})
	switch err := windows.ConnectNamedPipe(server, overlapped); err {
	case windows.ERROR_PIPE_CONNECTED:
		windows.SetEvent(event)
	case windows.ERROR_IO_PENDING, nil:
	default:
		windows.CloseHandle(event)
		windows.CloseHandle(server)
		t.Fatalf("named pipe connect: %v", err)
	}
	go func() {
		defer close(watched)
		windows.WaitForSingleObject(event, windows.INFINITE)
		close(opened)
	}()
	t.Cleanup(func() {
		// Closing the server ends the reader's read, with a broken pipe, and a connect still
		// pending; the event is set as well, for a connect that never came.
		windows.CloseHandle(server)
		windows.SetEvent(event)
		<-watched
		windows.CloseHandle(event)
		// The connect wrote into it until the server closed.
		runtime.KeepAlive(overlapped)
	})
	return filepath.ToSlash(name), opened
}
