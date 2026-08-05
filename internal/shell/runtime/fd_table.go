package runtime

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

const maxDescriptor = 255

var (
	errInvalidDescriptor     = errors.New("invalid file descriptor")
	errDescriptorAbsent      = errors.New("file descriptor is absent")
	errDescriptorClosed      = errors.New("file descriptor is closed")
	errDescriptorNotReadable = errors.New("file descriptor is not readable")
	errDescriptorNotWritable = errors.New("file descriptor is not writable")
)

type fdEntry struct {
	description *openDescription
	capability  fdCapability
}

type fdTable struct {
	// Callers own a table's lifecycle; mu prevents accidental concurrent mapping
	// mutation. Lock order is table mu before openDescription mu. I/O resources
	// are used after releasing mu; openDescription serializes shared writes.
	mu      sync.Mutex
	entries map[int]*fdEntry
}

func newFDTable(streams Streams) *fdTable {
	streams = fillStreams(streams)
	return &fdTable{entries: map[int]*fdEntry{
		0: {description: newBorrowedDescription(streams.Stdin, nil), capability: readable},
		1: {description: newBorrowedDescription(nil, streams.Stdout), capability: writable},
		2: {description: newBorrowedDescription(nil, streams.Stderr), capability: writable},
	}}
}

func (t *fdTable) bindBorrowed(fd int, resource io.ReadWriter, capability fdCapability) error {
	if err := validateDescriptor(fd); err != nil {
		return err
	}
	return t.rebind(fd, &fdEntry{
		description: newBorrowedDescription(resource, resource),
		capability:  capability,
	})
}

func (t *fdTable) bindOwned(fd int, resource io.ReadWriteCloser, capability fdCapability) error {
	// Ownership transfers on entry, including when descriptor validation fails.
	if err := validateDescriptor(fd); err != nil {
		if closeErr := resource.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("close rejected owned resource: %w", closeErr))
		}
		return err
	}
	return t.rebind(fd, &fdEntry{
		description: newOwnedDescription(resource),
		capability:  capability,
	})
}

func (t *fdTable) rebind(fd int, entry *fdEntry) error {
	if err := validateDescriptor(fd); err != nil {
		return err
	}
	t.mu.Lock()
	previous, exists := t.entries[fd]
	t.entries[fd] = entry
	t.mu.Unlock()
	if !exists || previous == nil {
		return nil
	}
	return previous.description.release()
}

func (t *fdTable) dup(target, source int) error {
	return t.alias(target, source, 0)
}

func (t *fdTable) alias(target, source int, capability fdCapability) error {
	if err := validateDescriptor(target); err != nil {
		return err
	}
	t.mu.Lock()
	entry, err := t.lookupLocked(source)
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("duplicate source %d: %w", source, err)
	}
	if target == source {
		if capability != 0 && entry.capability&capability == 0 {
			t.mu.Unlock()
			return capabilityError(capability)
		}
		t.mu.Unlock()
		return nil
	}
	if capability != 0 && entry.capability&capability == 0 {
		t.mu.Unlock()
		return capabilityError(capability)
	}
	if err := entry.description.retain(); err != nil {
		t.mu.Unlock()
		return err
	}
	previous, exists := t.entries[target]
	t.entries[target] = &fdEntry{description: entry.description, capability: entry.capability}
	t.mu.Unlock()
	if !exists || previous == nil {
		return nil
	}
	return previous.description.release()
}

func capabilityError(capability fdCapability) error {
	if capability == readable {
		return errDescriptorNotReadable
	}
	return errDescriptorNotWritable
}

func (t *fdTable) clone() (*fdTable, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entries := make(map[int]*fdEntry, len(t.entries))
	for fd, entry := range t.entries {
		if entry == nil {
			entries[fd] = nil
			continue
		}
		if err := entry.description.retain(); err != nil {
			clone := &fdTable{entries: entries}
			return nil, errors.Join(err, clone.closeAll())
		}
		entries[fd] = &fdEntry{description: entry.description, capability: entry.capability}
	}
	return &fdTable{entries: entries}, nil
}

func (t *fdTable) close(fd int) error {
	if err := validateDescriptor(fd); err != nil {
		return err
	}
	t.mu.Lock()
	entry, exists := t.entries[fd]
	t.entries[fd] = nil
	t.mu.Unlock()
	if !exists || entry == nil {
		return nil
	}
	return entry.description.release()
}

func (t *fdTable) closeAll() error {
	var closeErr error
	t.mu.Lock()
	descriptors := make([]int, 0, len(t.entries))
	for fd := range t.entries {
		descriptors = append(descriptors, fd)
	}
	t.mu.Unlock()
	slices.Sort(descriptors)
	for _, fd := range descriptors {
		closeErr = errors.Join(closeErr, t.close(fd))
	}
	return closeErr
}

func (t *fdTable) reader(fd int) (io.Reader, error) {
	entry, err := t.lookup(fd)
	if err != nil {
		return nil, err
	}
	if entry.capability&readable == 0 || entry.description.reader == nil {
		return nil, errDescriptorNotReadable
	}
	return entry.description.reader, nil
}

func (t *fdTable) writer(fd int) (io.Writer, error) {
	entry, err := t.lookup(fd)
	if err != nil {
		return nil, err
	}
	if entry.capability&writable == 0 || entry.description.writer == nil {
		return nil, errDescriptorNotWritable
	}
	return entry.description, nil
}

func (t *fdTable) lookup(fd int) (*fdEntry, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lookupLocked(fd)
}

func (t *fdTable) lookupLocked(fd int) (*fdEntry, error) {
	if err := validateDescriptor(fd); err != nil {
		return nil, err
	}
	entry, exists := t.entries[fd]
	if !exists {
		return nil, errDescriptorAbsent
	}
	if entry == nil {
		return nil, errDescriptorClosed
	}
	return entry, nil
}

func (t *fdTable) streams() Streams {
	return Streams{
		Stdin:  descriptorReader{table: t, fd: 0},
		Stdout: descriptorWriter{table: t, fd: 1},
		Stderr: descriptorWriter{table: t, fd: 2},
	}
}

func validateDescriptor(fd int) error {
	if fd < 0 || fd > maxDescriptor {
		return fmt.Errorf("descriptor %d: %w", fd, errInvalidDescriptor)
	}
	return nil
}
