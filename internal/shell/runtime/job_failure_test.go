package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

var (
	errInjectedClose = errors.New("injected close failure")
	errCleanupClose  = errors.New("cleanup close failure")
)

type failingReadWriteCloser struct {
	closed int
}

func (*failingReadWriteCloser) Read([]byte) (int, error)    { return 0, io.EOF }
func (*failingReadWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (c *failingReadWriteCloser) Close() error {
	c.closed++
	return errInjectedClose
}

type jobCountingCloser struct {
	closed   int
	closeErr error
}

func (*jobCountingCloser) Read([]byte) (int, error)    { return 0, io.EOF }
func (*jobCountingCloser) Write(p []byte) (int, error) { return len(p), nil }
func (c *jobCountingCloser) Close() error {
	c.closed++
	return c.closeErr
}

func TestRuntime_launchBackgroundClosesSnapshot_whenStdinReplacementFails(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})
	stdin := &failingReadWriteCloser{}
	stdout := &jobCountingCloser{closeErr: errCleanupClose}
	if err := rt.fds.bindOwned(0, stdin, readable); err != nil {
		t.Fatal(err)
	}
	if err := rt.fds.bindOwned(9, stdout, writable); err != nil {
		t.Fatal(err)
	}
	worker, err := rt.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.fds.close(0); err != nil {
		t.Fatal(err)
	}
	if err := rt.fds.close(9); err != nil {
		t.Fatal(err)
	}

	// When
	result := rt.launchBackgroundSnapshot(worker, func(Runtime) lineResult {
		return lineResult{status: 0}
	})

	// Then
	if result.status != 1 || !errors.Is(worker.jobScope.ctx.Err(), context.Canceled) || stdin.closed != 1 || stdout.closed != 1 {
		t.Fatalf("status = %d, worker context = %v, closes = stdin:%d stdout:%d", result.status, worker.jobScope.ctx.Err(), stdin.closed, stdout.closed)
	}
	if !strings.Contains(stderr.String(), errInjectedClose.Error()) || !strings.Contains(stderr.String(), errCleanupClose.Error()) {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if err := worker.fds.closeAll(); err != nil || stdin.closed != 1 || stdout.closed != 1 {
		t.Fatalf("second worker close = %v, closes = stdin:%d stdout:%d", err, stdin.closed, stdout.closed)
	}
}

func TestRuntime_launchBackgroundCancelsSnapshot_whenAdmissionFails(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})
	t.Cleanup(func() { _ = rt.fds.closeAll() })
	for range maxJobs {
		if _, err := rt.jobScope.register(); err != nil {
			t.Fatal(err)
		}
	}
	worker, err := rt.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runCalled := false

	// When
	result := rt.launchBackgroundSnapshot(worker, func(Runtime) lineResult {
		runCalled = true
		return lineResult{status: 0}
	})

	// Then
	if result.status != 1 || runCalled || !errors.Is(worker.jobScope.ctx.Err(), context.Canceled) {
		t.Fatalf("status = %d, run called = %t, worker context = %v", result.status, runCalled, worker.jobScope.ctx.Err())
	}
	if !strings.Contains(stderr.String(), errJobLimit.Error()) {
		t.Fatalf("stderr = %q, want job-limit diagnostic", stderr.String())
	}
	if err := worker.fds.closeAll(); err != nil {
		t.Fatalf("second worker close = %v", err)
	}
	records, ok := rt.jobScope.claimAll()
	if !ok || !rt.jobScope.consumeAll(records) {
		t.Fatal("failed to consume retained root records")
	}
}

func TestRuntime_jobsDisplaysCompletedStatus_atomically(t *testing.T) {
	// Given
	writer := &recordingWriter{}
	rt := New(applets.DefaultRegistry, Streams{Stdout: writer})
	first, _ := rt.jobScope.register()
	second, _ := rt.jobScope.register()
	rt.jobScope.complete(first, 0)
	rt.jobScope.complete(second, 1)

	// When
	status := rt.jobs(nil)

	// Then
	if status != 0 || len(writer.writes) != 2 || string(writer.writes[0]) != "[1] Done\n" || string(writer.writes[1]) != "[2] Done(1)\n" {
		t.Fatalf("status = %d, writes = %q", status, writer.writes)
	}
}

type recordingWriter struct {
	writes [][]byte
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, append([]byte(nil), p...))
	return len(p), nil
}

func TestRuntime_waitCancellationRetainsRecord(t *testing.T) {
	// Given
	rt := New(applets.DefaultRegistry, Streams{})
	record, _ := rt.jobScope.register()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	status := rt.wait(ctx, []string{"%1"})
	rt.jobScope.complete(record, 7)
	retryStatus := rt.wait(context.Background(), []string{"%1"})

	// Then
	if status != 1 || retryStatus != 7 || len(rt.jobScope.snapshot()) != 0 {
		t.Fatalf("statuses = %d, %d, records = %#v", status, retryStatus, rt.jobScope.snapshot())
	}
}

func TestRuntime_waitCancellationPrecedesCompletedRecord(t *testing.T) {
	// Given
	rt := New(applets.DefaultRegistry, Streams{})
	record, _ := rt.jobScope.register()
	rt.jobScope.complete(record, 7)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	status := rt.wait(ctx, []string{"%1"})
	retryStatus := rt.wait(context.Background(), []string{"%1"})

	// Then
	if status != 1 || retryStatus != 7 {
		t.Fatalf("statuses = %d, %d, want cancellation then cached completion", status, retryStatus)
	}
}
