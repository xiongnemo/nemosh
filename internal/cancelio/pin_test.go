package cancelio

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The wait is a read of a pipe nothing is written to, made plainly, so that nothing but the pin
// can end it: CancelSynchronousIo ends it on every Windows the tests run on, which an open an
// oplock holds was not; see internal/testutil/stall.

// silentPipe is a pipe whose reader waits, until the test ends and its writer closes.
func silentPipe(t *testing.T) *os.File {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close(); reader.Close() })
	return reader
}

// reads reads reader count times on a goroutine pinned with ctx, and reports each read's error.
func reads(ctx context.Context, reader *os.File, count int) <-chan error {
	results := make(chan error, count)
	go func() {
		defer Pin(ctx)()
		buffer := make([]byte, 16)
		for range count {
			_, err := reader.Read(buffer)
			results <- err
		}
	}()
	return results
}

// stillWaiting fails the test if a result arrives within 300 ms.
func stillWaiting(t *testing.T, results <-chan error, why string) {
	t.Helper()
	select {
	case err := <-results:
		t.Fatalf("read = %v; %s", err, why)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestPin_endsAReadItsContextHasEndedWhileItWaits(t *testing.T) {
	// Given: a read that waits on a pipe nothing is written to
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := reads(ctx, silentPipe(t), 1)
	stillWaiting(t, results, "it was meant to wait")

	// When
	cancel()

	// Then
	select {
	case err := <-results:
		if !errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
			t.Fatalf("read = %v, want ERROR_OPERATION_ABORTED", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the read still waits two seconds after its context ended")
	}
}

// Once the context has ended, the pin cancels again each interval -- a call cancelled is often
// followed by another, as os.Stat follows GetFileAttributesEx with CreateFile -- and no more once
// it is released. Whether Windows ends each wait is its own affair, which it does not always
// grant on the runners; that the pin asks each time is the pin's.
func TestPin_cancelsAgainEachIntervalUntilReleased(t *testing.T) {
	// Given: the cancels issued for one pinned thread, counted
	var counted atomic.Uint64
	var counting atomic.Uint32
	issued := func(thread uint32) {
		if thread == counting.Load() {
			counted.Add(1)
		}
	}
	cancelIssued.Store(&issued)
	t.Cleanup(func() { cancelIssued.Store(nil) })
	ctx, cancel := context.WithCancel(context.Background())
	release, released := make(chan struct{}), make(chan struct{})
	pinned := make(chan struct{})
	go func() {
		defer close(released)
		releasePin := Pin(ctx)
		counting.Store(windows.GetCurrentThreadId())
		close(pinned)
		<-release
		releasePin()
	}()
	<-pinned

	// When: the context ends, and 100 ms later the pin is released
	cancel()
	time.Sleep(100 * time.Millisecond)
	whileHeld := counted.Load()
	close(release)
	<-released
	afterRelease := counted.Load()
	time.Sleep(50 * time.Millisecond)

	// Then
	if whileHeld < 3 {
		t.Fatalf("%d cancels in the 100 ms after the context ended, want one each 10 ms", whileHeld)
	}
	if after := counted.Load(); after != afterRelease {
		t.Fatalf("%d cancels after the release, want none", after-afterRelease)
	}
}

func TestPin_cancelsNothingOnceReleased(t *testing.T) {
	// Given: a goroutine pinned with an ended context and released again
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := silentPipe(t)
	results := make(chan error, 1)
	go func() {
		Pin(ctx)()
		_, err := reader.Read(make([]byte, 16))
		results <- err
	}()

	// Then
	stillWaiting(t, results, "a released pin ended it")
}

func TestUnpinned_holdsThePinToTheThreadItResumesOn(t *testing.T) {
	// Given: a pinned goroutine that has waited unpinned, on a channel, and then reads
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := silentPipe(t)
	results := make(chan error, 1)
	go func() {
		defer Pin(ctx)()
		Unpinned(func() { time.Sleep(20 * time.Millisecond) })
		_, err := reader.Read(make([]byte, 16))
		results <- err
	}()
	stillWaiting(t, results, "it was meant to wait")

	// When
	cancel()

	// Then
	select {
	case err := <-results:
		if !errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
			t.Fatalf("read = %v, want ERROR_OPERATION_ABORTED", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the read still waits two seconds after its context ended")
	}
}

func TestUnpinned_cancelsNothingWhileItWaits(t *testing.T) {
	// Given: a pin whose context has ended, and a read made unpinned
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := silentPipe(t)
	results := make(chan error, 1)
	go func() {
		defer Pin(ctx)()
		Unpinned(func() {
			_, err := reader.Read(make([]byte, 16))
			results <- err
		})
	}()

	// Then
	stillWaiting(t, results, "a suspended pin ended it")
}

func TestCovers_isTheGoroutinePinnedWithThatContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	other, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()
	answers := make(chan [4]bool, 1)
	go func() {
		var got [4]bool
		got[0] = Covers(ctx)
		release := Pin(ctx)
		got[1] = Covers(ctx)
		got[2] = Covers(other)
		Unpinned(func() { got[3] = Covers(ctx) })
		release()
		answers <- got
	}()
	if got, want := <-answers, [4]bool{false, true, false, false}; got != want {
		t.Fatalf("Covers before the pin, with it, for another context, unpinned = %v, want %v", got, want)
	}
}

// BenchmarkPin is what a pin and its release cost, which a pipeline pays once a stage.
func BenchmarkPin(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.ReportAllocs()
	for b.Loop() {
		Pin(ctx)()
	}
}

// rereadSetting has NEMOSH_PIN read again at the next pin, and again after the test.
func rereadSetting(t *testing.T) {
	setting = struct {
		once sync.Once
		off  bool
	}{}
	t.Cleanup(func() {
		setting = struct {
			once sync.Once
			off  bool
		}{}
	})
}

func TestPin_offLeavesAWaitingReadWaiting(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_PIN", "off")
	rereadSetting(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := reads(ctx, silentPipe(t), 1)

	// When
	cancel()

	// Then
	stillWaiting(t, results, "under NEMOSH_PIN=off it was meant to wait")
}
