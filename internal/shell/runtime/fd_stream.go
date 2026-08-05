package runtime

import (
	"context"
	"os"
)

type descriptorReader struct {
	table *fdTable
	fd    int
}

func (r descriptorReader) Read(buffer []byte) (int, error) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return 0, err
	}
	return reader.Read(buffer)
}

func (r descriptorReader) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return 0, err
	}
	return readWithContext(ctx, reader, buffer)
}

func (r descriptorReader) LeaseStdinFile(ctx context.Context) (*os.File, func(), bool) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return nil, func() {}, false
	}
	if file, ok := reader.(*os.File); ok {
		return file, func() {}, true
	}
	leaser, ok := reader.(stdinFileLeaser)
	if !ok {
		return nil, func() {}, false
	}
	return leaser.LeaseStdinFile(ctx)
}

type descriptorWriter struct {
	table *fdTable
	fd    int
}

func (w descriptorWriter) Write(buffer []byte) (int, error) {
	writer, err := w.table.writer(w.fd)
	if err != nil {
		return 0, err
	}
	return writer.Write(buffer)
}
