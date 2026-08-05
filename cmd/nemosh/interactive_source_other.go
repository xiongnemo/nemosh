//go:build !windows

package main

import (
	"context"
	"io"
	"os"
)

type fileInputSource struct {
	reader io.Reader
	file   *os.File
}

func newFileInputSource(reader io.Reader, file *os.File) interactiveSource {
	return fileInputSource{reader: reader, file: file}
}

func (s fileInputSource) ReadContext(_ context.Context, buffer []byte) (int, error) {
	return s.reader.Read(buffer)
}

func (s fileInputSource) File() *os.File { return s.file }
func (fileInputSource) Close() error     { return nil }
