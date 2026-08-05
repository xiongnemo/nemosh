package runtime

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

var errDescriptionReleased = errors.New("open description already released")

type fdCapability uint8

const (
	readable fdCapability = 1 << iota
	writable
	readWrite = readable | writable
)

type openDescription struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	reader  io.Reader
	writer  io.Writer
	closer  io.Closer
	refs    int
}

func newBorrowedDescription(reader io.Reader, writer io.Writer) *openDescription {
	return &openDescription{reader: reader, writer: writer, refs: 1}
}

func newOwnedDescription(resource io.ReadWriteCloser) *openDescription {
	return &openDescription{reader: resource, writer: resource, closer: resource, refs: 1}
}

func (d *openDescription) Write(buffer []byte) (int, error) {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	return d.writer.Write(buffer)
}

func (d *openDescription) retain() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.refs == 0 {
		return errDescriptionReleased
	}
	d.refs++
	return nil
}

func (d *openDescription) release() error {
	d.mu.Lock()
	if d.refs == 0 {
		d.mu.Unlock()
		return errDescriptionReleased
	}
	d.refs--
	last := d.refs == 0
	closer := d.closer
	d.mu.Unlock()
	if !last || closer == nil {
		return nil
	}
	if err := closer.Close(); err != nil {
		return fmt.Errorf("close file description: %w", err)
	}
	return nil
}
