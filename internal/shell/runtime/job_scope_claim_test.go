package runtime

import (
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestJobScope_cancelledWaitRetainsClaimedRecordAndCapacity(t *testing.T) {
	// Given
	scope := newRootJobScope()
	record, err := scope.register()
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := scope.claim(record.id)
	if !ok {
		t.Fatal("claim failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	_, waitErr := waitJob(ctx, claimed)
	scope.release(claimed)

	// Then
	if waitErr == nil || len(scope.snapshot()) != 1 {
		t.Fatalf("wait error = %v, records = %d", waitErr, len(scope.snapshot()))
	}
	for range maxJobs - 1 {
		if _, err := scope.register(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := scope.register(); err == nil {
		t.Fatal("cancelled wait freed capacity")
	}
}

func TestRuntime_concurrentWaitersHaveSingleOwner(t *testing.T) {
	// Given
	rt := New(applets.DefaultRegistry, Streams{})
	record, err := rt.jobScope.register()
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	statuses := make(chan int, 2)
	for range 2 {
		go func() {
			<-start
			statuses <- rt.wait(context.Background(), []string{"%1"})
		}()
	}

	// When
	close(start)
	rt.jobScope.complete(record, 7)
	first := <-statuses
	second := <-statuses

	// Then
	if first+second != 9 || first == second {
		t.Fatalf("wait statuses = %d, %d, want 7 and 2", first, second)
	}
}

func TestJobScope_waitAllCapturesExactRecords(t *testing.T) {
	// Given
	scope := newRootJobScope()
	first, _ := scope.register()
	second, _ := scope.register()
	captured, ok := scope.claimAll()
	if !ok {
		t.Fatal("claim all failed")
	}
	third, err := scope.register()
	if err != nil {
		t.Fatal(err)
	}
	scope.complete(first, 0)
	scope.complete(second, 1)

	// When
	statuses, waitErr := waitJobs(context.Background(), captured)
	consumed := scope.consumeAll(captured)

	// Then
	if waitErr != nil || !consumed || len(statuses) != 2 || statuses[0] != 0 || statuses[1] != 1 {
		t.Fatalf("statuses = %v, error = %v, consumed = %t", statuses, waitErr, consumed)
	}
	remaining := scope.snapshot()
	if len(remaining) != 1 || remaining[0] != third {
		t.Fatalf("remaining = %#v, want exact third record", remaining)
	}
}
