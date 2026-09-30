package runtime

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
)

// An applet that names a device to write, `tee /dev/stderr`, `dd of=/dev/null`, `sort -o
// /dev/stdout`, opens it through the shell, as one that reads one does (device_input.go): a
// descriptor alias is the shell's own descriptor, and a virtual device is the one a redirection
// opens. Each was "not a host path", though `> /dev/null` took the same name.

// descriptionWriteLease is one of the shell's descriptors, lent to an applet that named it.
type descriptionWriteLease struct {
	description *openDescription
	once        sync.Once
	closed      atomic.Bool
	closeErr    error
}

func (l *descriptionWriteLease) Write(buffer []byte) (int, error) {
	if l.closed.Load() {
		return 0, errDescriptionReleased
	}
	return l.description.Write(buffer)
}

// Seek moves the descriptor's offset where it has one, a file, as lseek on the shell's own
// descriptor would, and fails as lseek does where it has none, a pipe.
func (l *descriptionWriteLease) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := l.description.writer.(io.Seeker); ok && !l.closed.Load() {
		return seeker.Seek(offset, whence)
	}
	return 0, syscall.ESPIPE
}

func (l *descriptionWriteLease) Close() error {
	l.closed.Store(true)
	l.once.Do(func() {
		l.closeErr = l.description.release()
	})
	return l.closeErr
}

func (t *fdTable) openWriterLease(fd int) (io.WriteCloser, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, err := t.lookupLocked(fd)
	if err != nil {
		return nil, err
	}
	if entry.capability&writable == 0 || entry.description.writer == nil {
		return nil, errDescriptorNotWritable
	}
	if err := entry.description.retain(); err != nil {
		return nil, err
	}
	return &descriptionWriteLease{description: entry.description}, nil
}

// OpenProcessOutput opens a path an applet writes to: a file, with flag and perm, or a device.
func (r Runtime) OpenProcessOutput(path string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	resolved, err := r.ResolveNemoshPath(path)
	if err != nil {
		return nil, err
	}
	if !resolved.Device {
		return os.OpenFile(resolved.Native, flag, perm)
	}
	device := string(resolved.Canonical)
	if fd, alias, err := deviceAlias(device); err != nil {
		return nil, err
	} else if alias {
		return r.fds.openWriterLease(fd)
	}
	return openOutputDevice(device, flag&os.O_APPEND != 0)
}
