package main

import (
	"context"
	"io"
	"os"
)

type contextInputReader interface {
	ReadContext(context.Context, []byte) (int, error)
}

type interactiveFileReader interface {
	InteractiveInputFile() *os.File
}

type interactiveSource interface {
	ReadContext(context.Context, []byte) (int, error)
	File() *os.File
	Close() error
}

type contextReaderSource struct {
	reader contextInputReader
	file   *os.File
}

func (s contextReaderSource) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	return s.reader.ReadContext(ctx, buffer)
}

func (s contextReaderSource) File() *os.File { return s.file }
func (contextReaderSource) Close() error     { return nil }

type blockingReaderSource struct {
	reader io.Reader
}

// ReadContext deliberately does not spawn a detached goroutine. Cancellation
// is guaranteed only for contextInputReader implementations and the Windows
// os.File adapter; an arbitrary blocking io.Reader can remain blocked.
func (s blockingReaderSource) ReadContext(_ context.Context, buffer []byte) (int, error) {
	return s.reader.Read(buffer)
}

func (blockingReaderSource) File() *os.File { return nil }
func (blockingReaderSource) Close() error   { return nil }

func newInteractiveSource(reader io.Reader) interactiveSource {
	file := interactiveInputFile(reader)
	if contextual, ok := reader.(contextInputReader); ok {
		return contextReaderSource{reader: contextual, file: file}
	}
	if file != nil {
		return newFileInputSource(reader, file)
	}
	return blockingReaderSource{reader: reader}
}

func interactiveInputFile(reader io.Reader) *os.File {
	if file, ok := reader.(*os.File); ok {
		return file
	}
	if backed, ok := reader.(interactiveFileReader); ok {
		return backed.InteractiveInputFile()
	}
	return nil
}
