package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Reading the bare `/dev` fails either way, and which failure depends on whose /dev it is.
//
// On Windows the shell provides the directory, so the refusal is its own typed one. Elsewhere the
// system provides it and the refusal comes from the platform -- opening a directory -- which is the
// right answer there and not one this shell should be inventing a message for.
func TestP05WaveA_OpenProcessInput_rejectsExactDev(t *testing.T) {
	// Given
	runtime := New(applets.DefaultRegistry, Streams{})

	// When
	input, err := runtime.OpenProcessInput("/dev")

	// Then
	if err == nil {
		if input != nil {
			_ = input.Close()
		}
		t.Fatal("reading the bare /dev succeeded; it is a directory on every platform")
	}
	if runtimeProvidesDev && !errors.Is(err, errUnsupportedDevice) {
		t.Fatalf("exact /dev error: got %v want %v", err, errUnsupportedDevice)
	}
}

func TestP05WaveA_readerLease_retainsOwnedDescriptionAndClosesExactlyOnce(t *testing.T) {
	// Given
	resource := &countingInputCloser{Reader: bytes.NewBufferString("retained")}
	table := newFDTable(Streams{})
	if err := table.bindOwnedReader(7, resource); err != nil {
		t.Fatalf("bind reader: %v", err)
	}
	lease, err := table.openReaderLease(7)
	if err != nil {
		t.Fatalf("open reader lease: %v", err)
	}

	// When
	if err := table.close(7); err != nil {
		t.Fatalf("close table entry: %v", err)
	}
	data, readErr := io.ReadAll(lease)
	firstCloseErr := lease.Close()
	secondCloseErr := lease.Close()

	// Then
	if readErr != nil || string(data) != "retained" {
		t.Fatalf("leased read: data=%q error=%v", data, readErr)
	}
	if firstCloseErr != nil || secondCloseErr != nil {
		t.Fatalf("lease closes: first=%v second=%v", firstCloseErr, secondCloseErr)
	}
	if resource.closes != 1 {
		t.Fatalf("owned resource close count: got %d want 1", resource.closes)
	}
}

func TestP05WaveA_readerLease_rejectsReadAfterClose(t *testing.T) {
	// Given
	table := newFDTable(Streams{Stdin: bytes.NewBufferString("stale")})
	lease, err := table.openReaderLease(0)
	if err != nil {
		t.Fatalf("open reader lease: %v", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatalf("close reader lease: %v", err)
	}

	// When
	buffer := make([]byte, 1)
	count, readErr := lease.Read(buffer)
	contextual, ok := lease.(interface {
		ReadContext(context.Context, []byte) (int, error)
	})
	if !ok {
		t.Fatal("lease dropped the ReadContext capability")
	}
	contextCount, contextErr := contextual.ReadContext(context.Background(), buffer)

	// Then
	if count != 0 || !errors.Is(readErr, errDescriptionReleased) {
		t.Fatalf("read after close: count=%d error=%v", count, readErr)
	}
	if contextCount != 0 || !errors.Is(contextErr, errDescriptionReleased) {
		t.Fatalf("contextual read after close: count=%d error=%v", contextCount, contextErr)
	}
}

func TestP05WaveA_readerLease_closeDoesNotWaitForBlockingRead(t *testing.T) {
	// Given
	reader := &blockingLeaseReader{started: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	releaseReader := func() { releaseOnce.Do(func() { close(reader.release) }) }
	t.Cleanup(releaseReader)
	table := newFDTable(Streams{Stdin: reader})
	lease, err := table.openReaderLease(0)
	if err != nil {
		t.Fatalf("open reader lease: %v", err)
	}
	readDone := make(chan error, 1)
	go func() {
		_, readErr := lease.Read(make([]byte, 1))
		readDone <- readErr
	}()
	awaitSignal(t, reader.started, "blocking read start")

	// When
	closeDone := make(chan error, 1)
	go func() { closeDone <- lease.Close() }()
	closeErr := awaitError(t, closeDone, "lease close")

	// Then
	if closeErr != nil {
		t.Fatalf("close reader lease: %v", closeErr)
	}
	releaseReader()
	if readErr := awaitError(t, readDone, "blocking read completion"); readErr != nil {
		t.Fatalf("in-flight read: %v", readErr)
	}
}

func TestP05WaveA_ownedReaderLease_closeInterruptsBlockingReadExactlyOnce(t *testing.T) {
	// Given
	interruptErr := errors.New("read interrupted by close")
	resource := &interruptibleOwnedReader{
		started:      make(chan struct{}),
		interrupted:  make(chan struct{}),
		interruptErr: interruptErr,
	}
	table := newFDTable(Streams{})
	if err := table.bindOwnedReader(7, resource); err != nil {
		t.Fatalf("bind owned reader: %v", err)
	}
	lease, err := table.openReaderLease(7)
	if err != nil {
		t.Fatalf("open owned reader lease: %v", err)
	}
	if err := table.close(7); err != nil {
		t.Fatalf("release table reference: %v", err)
	}
	readDone := make(chan error, 1)
	go func() {
		_, readErr := lease.Read(make([]byte, 1))
		readDone <- readErr
	}()
	awaitSignal(t, resource.started, "owned blocking read start")

	// When
	closeDone := make(chan error, 1)
	go func() { closeDone <- lease.Close() }()
	firstCloseErr := awaitError(t, closeDone, "owned lease close")
	secondCloseErr := lease.Close()
	inFlightReadErr := awaitError(t, readDone, "owned blocking read completion")
	_, postCloseReadErr := lease.Read(make([]byte, 1))

	// Then
	if firstCloseErr != nil || secondCloseErr != nil {
		t.Fatalf("owned lease closes: first=%v second=%v", firstCloseErr, secondCloseErr)
	}
	if !errors.Is(inFlightReadErr, interruptErr) {
		t.Fatalf("in-flight read error: got %v want %v", inFlightReadErr, interruptErr)
	}
	if !errors.Is(postCloseReadErr, errDescriptionReleased) {
		t.Fatalf("post-close read error: got %v want %v", postCloseReadErr, errDescriptionReleased)
	}
	if got := resource.closes.Load(); got != 1 {
		t.Fatalf("owned resource close count: got %d want 1", got)
	}
}

func TestP05WaveA_ownedReaderLease_concurrentClosePublishesErrorAndClosesExactlyOnce(t *testing.T) {
	// Given
	closeFailure := errors.New("owned close failed")
	resource := &interruptibleOwnedReader{
		started:     make(chan struct{}),
		interrupted: make(chan struct{}),
		closeErr:    closeFailure,
	}
	table := newFDTable(Streams{})
	if err := table.bindOwnedReader(7, resource); err != nil {
		t.Fatalf("bind owned reader: %v", err)
	}
	lease, err := table.openReaderLease(7)
	if err != nil {
		t.Fatalf("open owned reader lease: %v", err)
	}
	if err := table.close(7); err != nil {
		t.Fatalf("release table reference: %v", err)
	}
	const callers = 8
	start := make(chan struct{})
	closeResults := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			closeResults <- lease.Close()
		}()
	}

	// When
	close(start)
	for range callers {
		closeErr := awaitError(t, closeResults, "concurrent lease close")
		if !errors.Is(closeErr, closeFailure) {
			t.Fatalf("concurrent close error: got %v want %v", closeErr, closeFailure)
		}
	}

	// Then
	if got := resource.closes.Load(); got != 1 {
		t.Fatalf("owned resource close count: got %d want 1", got)
	}
}

type countingInputCloser struct {
	io.Reader
	closes int
}

type blockingLeaseReader struct {
	started chan struct{}
	release chan struct{}
}

type interruptibleOwnedReader struct {
	started      chan struct{}
	interrupted  chan struct{}
	interruptErr error
	closeErr     error
	closeOnce    sync.Once
	closes       atomic.Int32
}

func (r *blockingLeaseReader) Read(buffer []byte) (int, error) {
	close(r.started)
	<-r.release
	buffer[0] = 'x'
	return 1, nil
}

func (r *interruptibleOwnedReader) Read([]byte) (int, error) {
	close(r.started)
	<-r.interrupted
	return 0, r.interruptErr
}

func (r *interruptibleOwnedReader) Close() error {
	r.closes.Add(1)
	r.closeOnce.Do(func() { close(r.interrupted) })
	return r.closeErr
}

func awaitSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
	}
}

func awaitError(t *testing.T, result <-chan error, operation string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
		return nil
	}
}

func (r *countingInputCloser) Close() error {
	r.closes++
	return nil
}
