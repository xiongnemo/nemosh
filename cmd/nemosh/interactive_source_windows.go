//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

var cancelSynchronousIO = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

type windowsFileSource struct {
	file     *os.File
	requests chan windowsReadRequest
	done     chan struct{}
	thread   windows.Handle
	startErr error
}

type windowsReadRequest struct {
	buffer []byte
	result chan windowsReadResult
}

type windowsReadResult struct {
	count int
	err   error
}

type windowsReaderReady struct {
	thread windows.Handle
	err    error
}

func newFileInputSource(reader io.Reader, file *os.File) interactiveSource {
	requests := make(chan windowsReadRequest)
	done := make(chan struct{})
	ready := make(chan windowsReaderReady, 1)
	go serveWindowsFileReads(reader, requests, ready, done)
	started := <-ready
	return &windowsFileSource{
		file: file, requests: requests, done: done,
		thread: started.thread, startErr: started.err,
	}
}

func (s *windowsFileSource) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	if s.startErr != nil {
		return 0, s.startErr
	}
	result := make(chan windowsReadResult, 1)
	request := windowsReadRequest{buffer: buffer, result: result}
	select {
	case s.requests <- request:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case read := <-result:
		return read.count, read.err
	case <-ctx.Done():
		cancelErr := cancelWindowsSynchronousRead(s.thread, result)
		if cancelErr != nil {
			return 0, errors.Join(ctx.Err(), cancelErr)
		}
		return 0, ctx.Err()
	}
}

func (s *windowsFileSource) File() *os.File { return s.file }

func (s *windowsFileSource) Close() error {
	if s.startErr != nil {
		return nil
	}
	close(s.requests)
	<-s.done
	if err := windows.CloseHandle(s.thread); err != nil {
		return fmt.Errorf("close interactive input thread: %w", err)
	}
	return nil
}

func serveWindowsFileReads(
	reader io.Reader,
	requests <-chan windowsReadRequest,
	ready chan<- windowsReaderReady,
	done chan<- struct{},
) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)
	var thread windows.Handle
	err := windows.DuplicateHandle(
		windows.CurrentProcess(), windows.CurrentThread(), windows.CurrentProcess(),
		&thread, 0, false, windows.DUPLICATE_SAME_ACCESS,
	)
	ready <- windowsReaderReady{thread: thread, err: err}
	if err != nil {
		return
	}
	for request := range requests {
		count, readErr := reader.Read(request.buffer)
		request.result <- windowsReadResult{count: count, err: readErr}
	}
}

func cancelWindowsSynchronousRead(thread windows.Handle, completed <-chan windowsReadResult) error {
	for {
		select {
		case <-completed:
			return nil
		default:
		}
		result, _, callErr := cancelSynchronousIO.Call(uintptr(thread))
		if result != 0 {
			<-completed
			return nil
		}
		if !errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			<-completed
			return fmt.Errorf("cancel interactive input read: %w", callErr)
		}
		runtime.Gosched()
	}
}
