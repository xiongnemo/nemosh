package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestFDTable_bindOwnedTakesOwnershipWhenDescriptorInvalid(t *testing.T) {
	resource := &countingReadWriteCloser{}
	table := newFDTable(Streams{})

	err := table.bindOwned(256, resource, readWrite)

	if !errors.Is(err, errInvalidDescriptor) {
		t.Fatalf("invalid owned bind: %v", err)
	}
	if resource.closes != 1 {
		t.Fatalf("invalid owned bind close count: %d", resource.closes)
	}
}

func TestFDTable_dupSelfPreservesOpenDescriptor(t *testing.T) {
	resource := &countingReadWriteCloser{}
	table := newFDTable(Streams{})
	if err := table.bindOwned(50, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}

	if err := table.dup(50, 50); err != nil {
		t.Fatalf("dup self: %v", err)
	}
	if resource.closes != 0 {
		t.Fatalf("self dup closed resource: %d", resource.closes)
	}
	if _, err := table.writer(50); err != nil {
		t.Fatalf("writer after self dup: %v", err)
	}
}

func TestFDTable_dupSelfReturnsClosedError(t *testing.T) {
	table := newFDTable(Streams{})
	if err := table.close(51); err != nil {
		t.Fatalf("close descriptor: %v", err)
	}

	err := table.dup(51, 51)

	if !errors.Is(err, errDescriptorClosed) {
		t.Fatalf("dup closed self: %v", err)
	}
}

func TestFDTable_dupInvalidSourcePreservesExistingTarget(t *testing.T) {
	target := &bytes.Buffer{}
	table := newFDTable(Streams{})
	if err := table.bindBorrowed(52, target, writable); err != nil {
		t.Fatalf("bind target: %v", err)
	}

	err := table.dup(52, 256)

	if !errors.Is(err, errInvalidDescriptor) {
		t.Fatalf("dup invalid source: %v", err)
	}
	writer, writerErr := table.writer(52)
	if writerErr != nil {
		t.Fatalf("target writer: %v", writerErr)
	}
	if _, writerErr := writer.Write([]byte("preserved")); writerErr != nil {
		t.Fatalf("write target: %v", writerErr)
	}
	if target.String() != "preserved" {
		t.Fatalf("invalid source replaced existing target: %q", target.String())
	}
}

func TestFDTable_cloneReleasesOwnedDescriptionWhenParentClosesFirst(t *testing.T) {
	assertCloneReleaseOrder(t, true)
}

func TestFDTable_cloneReleasesOwnedDescriptionWhenCloneClosesFirst(t *testing.T) {
	assertCloneReleaseOrder(t, false)
}

func TestFDTable_closeErrorOccursExactlyOnce(t *testing.T) {
	closeErr := errors.New("close failed")
	resource := &countingReadWriteCloser{closeErr: closeErr}
	table := newFDTable(Streams{})
	if err := table.bindOwned(54, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}

	firstErr := table.close(54)
	secondErr := table.close(54)

	if !errors.Is(firstErr, closeErr) {
		t.Fatalf("first close error: %v", firstErr)
	}
	if secondErr != nil {
		t.Fatalf("second close: %v", secondErr)
	}
	if resource.closes != 1 {
		t.Fatalf("close count: %d", resource.closes)
	}
}

func TestOpenDescription_releaseRejectsReferenceUnderflow(t *testing.T) {
	description := newBorrowedDescription(bytes.NewBuffer(nil), nil)
	if err := description.release(); err != nil {
		t.Fatalf("first release: %v", err)
	}

	err := description.release()

	if !errors.Is(err, errDescriptionReleased) {
		t.Fatalf("release underflow: %v", err)
	}
}

func TestOpenDescription_retainRejectsReleasedDescription(t *testing.T) {
	description := newBorrowedDescription(bytes.NewBuffer(nil), nil)
	if err := description.release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	if err := description.retain(); !errors.Is(err, errDescriptionReleased) {
		t.Fatalf("retain released: %v", err)
	}
}

func TestFDTable_cloneAndFinalCloseNeverResurrectDescription(t *testing.T) {
	for range 100 {
		resource := &synchronizedCountingCloser{}
		table := newFDTable(Streams{})
		if err := table.bindOwned(55, resource, readWrite); err != nil {
			t.Fatalf("bind: %v", err)
		}
		start := make(chan struct{})
		var clone *fdTable
		var cloneErr error
		var closeErr error
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			clone, cloneErr = table.clone()
		}()
		go func() {
			defer wait.Done()
			<-start
			closeErr = table.closeAll()
		}()
		close(start)
		wait.Wait()
		if closeErr != nil {
			t.Fatalf("close: %v", closeErr)
		}
		if cloneErr == nil {
			if err := clone.closeAll(); err != nil {
				t.Fatalf("clone close: %v", err)
			}
		} else if !errors.Is(cloneErr, errDescriptionReleased) {
			t.Fatalf("clone: %v", cloneErr)
		}
		if got := resource.CloseCount(); got != 1 {
			t.Fatalf("close count: %d", got)
		}
	}
}

func TestFDTable_closeAllUsesDescriptorOrder(t *testing.T) {
	order := make([]int, 0, 3)
	table := newFDTable(Streams{})
	for _, fd := range []int{60, 58, 59} {
		resource := &orderedCloser{fd: fd, order: &order}
		if err := table.bindOwned(fd, resource, readWrite); err != nil {
			t.Fatalf("bind fd %d: %v", fd, err)
		}
	}

	if err := table.closeAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
	if got, want := fmt.Sprint(order), "[58 59 60]"; got != want {
		t.Fatalf("close order: got %s want %s", got, want)
	}
}

func assertCloneReleaseOrder(t *testing.T, parentFirst bool) {
	t.Helper()
	resource := &countingReadWriteCloser{}
	parent := newFDTable(Streams{})
	if err := parent.bindOwned(53, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}
	clone, err := parent.clone()
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	first, second := parent, clone
	if !parentFirst {
		first, second = clone, parent
	}
	if err := first.closeAll(); err != nil {
		t.Fatalf("close first table: %v", err)
	}
	if resource.closes != 0 {
		t.Fatalf("resource closed before final table: %d", resource.closes)
	}
	if err := second.closeAll(); err != nil {
		t.Fatalf("close second table: %v", err)
	}
	if resource.closes != 1 {
		t.Fatalf("final close count: %d", resource.closes)
	}
}

type orderedCloser struct {
	bytes.Buffer
	fd    int
	order *[]int
}

type synchronizedCountingCloser struct {
	bytes.Buffer
	mu     sync.Mutex
	closes int
}

func (c *synchronizedCountingCloser) Close() error {
	c.mu.Lock()
	c.closes++
	c.mu.Unlock()
	return nil
}

func (c *synchronizedCountingCloser) CloseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func (c *orderedCloser) Close() error {
	*c.order = append(*c.order, c.fd)
	return nil
}
