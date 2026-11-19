package cancelio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/xiongnemo/nemosh/internal/testutil/oplock"
)

// heldFile is a file an open of waits on, and the channel that says one is waiting.
func heldFile(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "held.txt")
	if err := os.WriteFile(path, []byte("held\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, oplock.Hold(t, path)
}

// opens opens path count times on a goroutine pinned with ctx, and reports each open's error.
func opens(ctx context.Context, path string, count int) <-chan error {
	results := make(chan error, count)
	go func() {
		release := Pin(ctx)
		defer release()
		for range count {
			file, err := os.Open(path)
			if err == nil {
				file.Close()
			}
			results <- err
		}
	}()
	return results
}

func awaitWaiting(t *testing.T, breaking <-chan struct{}) {
	t.Helper()
	select {
	case <-breaking:
	case <-time.After(5 * time.Second):
		t.Fatal("no open arrived to wait on the oplock")
	}
}

func TestPin_endsEachOpenItsContextHasEndedWhileItWaits(t *testing.T) {
	// Given: an open that waits for an oplock's holder, which never answers, and two after it
	path, breaking := heldFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := opens(ctx, path, 3)
	awaitWaiting(t, breaking)

	// When
	cancel()

	// Then: each is aborted, the second and third as they start to wait
	deadline := time.After(2 * time.Second)
	for index := range 3 {
		select {
		case err := <-results:
			if !errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
				t.Fatalf("open %d: %v, want ERROR_OPERATION_ABORTED", index+1, err)
			}
		case <-deadline:
			t.Fatalf("open %d still waits two seconds after its context ended", index+1)
		}
	}
}

func TestPin_cancelsNothingOnceReleased(t *testing.T) {
	// Given: a goroutine pinned with an ended context and released again
	path, breaking := heldFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := make(chan error, 1)
	go func() {
		Pin(ctx)()
		file, err := os.Open(path)
		if err == nil {
			file.Close()
		}
		results <- err
	}()

	// When
	awaitWaiting(t, breaking)

	// Then
	select {
	case err := <-results:
		t.Fatalf("open = %v; a released pin ended it", err)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestUnpinned_holdsThePinToTheThreadItResumesOn(t *testing.T) {
	// Given: a pinned goroutine that has waited unpinned, on a channel, and then opens
	path, breaking := heldFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan error, 1)
	go func() {
		defer Pin(ctx)()
		Unpinned(func() { time.Sleep(20 * time.Millisecond) })
		file, err := os.Open(path)
		if err == nil {
			file.Close()
		}
		results <- err
	}()
	awaitWaiting(t, breaking)

	// When
	cancel()

	// Then
	select {
	case err := <-results:
		if !errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
			t.Fatalf("open = %v, want ERROR_OPERATION_ABORTED", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the open still waits two seconds after its context ended")
	}
}

func TestUnpinned_cancelsNothingWhileItWaits(t *testing.T) {
	// Given: a pin whose context has ended, and an open made unpinned
	path, breaking := heldFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := make(chan error, 1)
	go func() {
		defer Pin(ctx)()
		Unpinned(func() {
			file, err := os.Open(path)
			if err == nil {
				file.Close()
			}
			results <- err
		})
	}()

	// When
	awaitWaiting(t, breaking)

	// Then
	select {
	case err := <-results:
		t.Fatalf("open = %v; a suspended pin ended it", err)
	case <-time.After(300 * time.Millisecond):
	}
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

func TestPin_offLeavesAWaitingOpenWaiting(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_PIN", "off")
	rereadSetting(t)
	path, breaking := heldFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := opens(ctx, path, 1)
	awaitWaiting(t, breaking)

	// When
	cancel()

	// Then
	select {
	case err := <-results:
		t.Fatalf("open = %v under NEMOSH_PIN=off, want it still waiting", err)
	case <-time.After(300 * time.Millisecond):
	}
}
