package runtime

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestFDTable_initializesBorrowedStandardDescriptorsFromStreams(t *testing.T) {
	in := bytes.NewBufferString("input")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	table := newFDTable(Streams{Stdin: in, Stdout: out, Stderr: errOut})

	streams := table.streams()

	got, err := io.ReadAll(streams.Stdin)
	if err != nil {
		t.Fatalf("read projected stdin: %v", err)
	}
	if string(got) != "input" {
		t.Fatalf("projected stdin: got %q", got)
	}
	if _, err := io.WriteString(streams.Stdout, "out"); err != nil {
		t.Fatalf("write projected stdout: %v", err)
	}
	if _, err := io.WriteString(streams.Stderr, "err"); err != nil {
		t.Fatalf("write projected stderr: %v", err)
	}
	if out.String() != "out" || errOut.String() != "err" {
		t.Fatalf("projected output did not reach constructor streams")
	}
	if err := table.closeAll(); err != nil {
		t.Fatalf("close borrowed descriptors: %v", err)
	}
}

func TestFDTable_enforcesDescriptorRange(t *testing.T) {
	table := newFDTable(Streams{})

	for _, fd := range []int{-1, 256} {
		if err := table.bindBorrowed(fd, bytes.NewBuffer(nil), readable); !errors.Is(err, errInvalidDescriptor) {
			t.Fatalf("bind fd %d: expected invalid descriptor, got %v", fd, err)
		}
	}
	if err := table.bindBorrowed(255, bytes.NewBuffer(nil), readable); err != nil {
		t.Fatalf("bind fd 255: %v", err)
	}
}

func TestFDTable_tracksReadableAndWritableCapabilities(t *testing.T) {
	table := newFDTable(Streams{})
	if err := table.bindBorrowed(10, bytes.NewBuffer(nil), readable); err != nil {
		t.Fatalf("bind readable: %v", err)
	}
	if err := table.bindBorrowed(11, &bytes.Buffer{}, writable); err != nil {
		t.Fatalf("bind writable: %v", err)
	}

	if _, err := table.writer(10); !errors.Is(err, errDescriptorNotWritable) {
		t.Fatalf("writer on readable fd: %v", err)
	}
	if _, err := table.reader(11); !errors.Is(err, errDescriptorNotReadable) {
		t.Fatalf("reader on writable fd: %v", err)
	}
}

func TestFDTable_distinguishesAbsentAndClosedDescriptors(t *testing.T) {
	table := newFDTable(Streams{})
	if err := table.bindBorrowed(40, bytes.NewBuffer(nil), readable); err != nil {
		t.Fatalf("bind descriptor: %v", err)
	}
	if err := table.close(40); err != nil {
		t.Fatalf("close descriptor: %v", err)
	}

	if _, err := table.reader(41); !errors.Is(err, errDescriptorAbsent) {
		t.Fatalf("absent descriptor: %v", err)
	}
	if _, err := table.reader(40); !errors.Is(err, errDescriptorClosed) {
		t.Fatalf("closed descriptor: %v", err)
	}
}

func TestFDTable_neverClosesBorrowedResources(t *testing.T) {
	resource := &countingReadWriteCloser{}
	table := newFDTable(Streams{})
	if err := table.bindBorrowed(15, resource, readWrite); err != nil {
		t.Fatalf("bind borrowed: %v", err)
	}

	if err := table.close(15); err != nil {
		t.Fatalf("close borrowed descriptor: %v", err)
	}
	if resource.closes != 0 {
		t.Fatalf("borrowed resource close count: %d", resource.closes)
	}
}

func TestFDTable_dupCapturesCurrentOpenDescription(t *testing.T) {
	first := &bytes.Buffer{}
	second := &bytes.Buffer{}
	table := newFDTable(Streams{})
	if err := table.bindBorrowed(9, first, writable); err != nil {
		t.Fatalf("bind first: %v", err)
	}
	if err := table.dup(12, 9); err != nil {
		t.Fatalf("dup: %v", err)
	}
	if err := table.bindBorrowed(9, second, writable); err != nil {
		t.Fatalf("rebind source: %v", err)
	}

	alias, err := table.writer(12)
	if err != nil {
		t.Fatalf("writer alias: %v", err)
	}
	if _, err := io.WriteString(alias, "alias"); err != nil {
		t.Fatalf("write alias: %v", err)
	}
	if first.String() != "alias" || second.Len() != 0 {
		t.Fatalf("dup did not retain original description")
	}
}

func TestFDTable_rebindReleasesPreviousOwnedDescription(t *testing.T) {
	previous := &countingReadWriteCloser{}
	table := newFDTable(Streams{})
	if err := table.bindOwned(14, previous, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}

	if err := table.bindBorrowed(14, bytes.NewBuffer(nil), readable); err != nil {
		t.Fatalf("rebind borrowed: %v", err)
	}
	if previous.closes != 1 {
		t.Fatalf("previous owned close count: %d", previous.closes)
	}
}

func TestFDTable_cloneIsolatesMappingsAndSharesOpenDescriptions(t *testing.T) {
	shared := &bytes.Buffer{}
	parent := newFDTable(Streams{})
	if err := parent.bindBorrowed(20, shared, writable); err != nil {
		t.Fatalf("bind parent: %v", err)
	}
	child, err := parent.clone()
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if err := parent.close(20); err != nil {
		t.Fatalf("close parent mapping: %v", err)
	}

	if _, err := parent.writer(20); !errors.Is(err, errDescriptorClosed) {
		t.Fatalf("parent closed descriptor: %v", err)
	}
	writer, err := child.writer(20)
	if err != nil {
		t.Fatalf("child writer: %v", err)
	}
	if _, err := io.WriteString(writer, "child"); err != nil {
		t.Fatalf("write child: %v", err)
	}
	if shared.String() != "child" {
		t.Fatalf("clone did not share description")
	}
}

func TestFDTable_closesOwnedResourceExactlyOnceAfterLastReference(t *testing.T) {
	resource := &countingReadWriteCloser{}
	table := newFDTable(Streams{})
	if err := table.bindOwned(30, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}
	if err := table.dup(31, 30); err != nil {
		t.Fatalf("dup owned: %v", err)
	}
	clone, err := table.clone()
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	if err := table.closeAll(); err != nil {
		t.Fatalf("close table: %v", err)
	}
	if resource.closes != 0 {
		t.Fatalf("owned resource closed before clone released: %d", resource.closes)
	}
	if err := clone.closeAll(); err != nil {
		t.Fatalf("close clone: %v", err)
	}
	if resource.closes != 1 {
		t.Fatalf("owned resource close count: %d", resource.closes)
	}
}

func TestFDTable_stdioProjectionUsesClosedDescriptorAdapters(t *testing.T) {
	table := newFDTable(Streams{})
	if err := table.close(0); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if err := table.close(1); err != nil {
		t.Fatalf("close stdout: %v", err)
	}

	streams := table.streams()
	if _, err := streams.Stdin.Read(make([]byte, 1)); !errors.Is(err, errDescriptorClosed) {
		t.Fatalf("closed stdin read: %v", err)
	}
	if _, err := streams.Stdout.Write([]byte("x")); !errors.Is(err, errDescriptorClosed) {
		t.Fatalf("closed stdout write: %v", err)
	}
}

type countingReadWriteCloser struct {
	bytes.Buffer
	closes   int
	closeErr error
}

func (c *countingReadWriteCloser) Close() error {
	c.closes++
	return c.closeErr
}
