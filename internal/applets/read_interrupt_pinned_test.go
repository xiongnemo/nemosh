package applets

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/cancelio"
)

// A goroutine pinned with the read's context reads a pipe with no watcher of its own: the pin
// ends a read that is blocked when the context ends, and a watcher was a goroutine and two
// channels at each read, whose wait a pinned goroutine paid a thread's handoff for, 12.7 us a
// read -- `while read` over 20,000 lines on a pipe took 4.9 times as long.

func TestReadInterruptibly_pinnedReadsAPipeWithNoWatcher(t *testing.T) {
	if raceDetector {
		t.Skip("allocations are counted without the race detector, which makes its own")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	byte1, buffer := []byte{'x'}, make([]byte, 1)
	allocations := make(chan float64, 1)
	go func() {
		defer cancelio.Pin(ctx)()
		allocations <- testing.AllocsPerRun(100, func() {
			writer.Write(byte1)
			ReadInterruptibly(ctx, reader, buffer)
		})
	}()
	if got := <-allocations; got != 0 {
		t.Fatalf("a pinned read of a pipe made %v allocations, want none: no watcher", got)
	}
}

func TestReadInterruptibly_pinnedEndsABlockedReadWhenItsContextEnds(t *testing.T) {
	// Given: a pinned read of a pipe nothing is written to
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan error, 1)
	go func() {
		defer cancelio.Pin(ctx)()
		_, err := ReadInterruptibly(ctx, reader, make([]byte, 16))
		results <- err
	}()
	select {
	case err := <-results:
		t.Fatalf("the read returned %v on its own, so this cannot measure cancellation", err)
	case <-time.After(100 * time.Millisecond):
	}

	// When
	cancel()

	// Then
	select {
	case err := <-results:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the read answered %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the pinned read still waits two seconds after its context ended")
	}
}
