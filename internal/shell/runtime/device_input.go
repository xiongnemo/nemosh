package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

type descriptionReadLease struct {
	reader      io.Reader
	description *openDescription
	once        sync.Once
	closed      atomic.Bool
	closeErr    error
}

func (l *descriptionReadLease) Read(buffer []byte) (int, error) {
	if l.closed.Load() {
		return 0, errDescriptionReleased
	}
	return l.reader.Read(buffer)
}

func (l *descriptionReadLease) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	if l.closed.Load() {
		return 0, errDescriptionReleased
	}
	return readWithContext(ctx, l.reader, buffer)
}

func (l *descriptionReadLease) Close() error {
	l.closed.Store(true)
	l.once.Do(func() {
		l.closeErr = l.description.release()
	})
	return l.closeErr
}

func (t *fdTable) openReaderLease(fd int) (io.ReadCloser, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, err := t.lookupLocked(fd)
	if err != nil {
		return nil, err
	}
	if entry.capability&readable == 0 || entry.description.reader == nil {
		return nil, errDescriptorNotReadable
	}
	if err := entry.description.retain(); err != nil {
		return nil, err
	}
	return &descriptionReadLease{reader: entry.description.reader, description: entry.description}, nil
}

func (r Runtime) OpenProcessInput(path string) (io.ReadCloser, error) {
	resolved, err := r.ResolveNemoshPath(path)
	if err != nil {
		return nil, err
	}
	if !resolved.Device {
		return os.Open(resolved.Native)
	}
	device := string(resolved.Canonical)
	if source, alias, err := deviceAlias(device); err != nil {
		return nil, err
	} else if alias {
		return r.fds.openReaderLease(source)
	}
	if isVirtualDevice(device) {
		return openInputRedirect(device, Streams{})
	}
	return nil, fmt.Errorf("%s: %w", device, errUnsupportedDevice)
}
