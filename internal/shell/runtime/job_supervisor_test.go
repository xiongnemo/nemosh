package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestJobSupervisor_concurrentCrossScopeAdmissionTotalsMaxJobs(t *testing.T) {
	// Given
	root, scopes := newJobScopeFamily(t, 7)
	start := make(chan struct{})
	results := make(chan registeredJob, maxJobs*2)
	var ready sync.WaitGroup
	ready.Add(maxJobs * 2)
	for index := range maxJobs * 2 {
		scope := scopes[index%len(scopes)]
		go func() {
			ready.Done()
			<-start
			record, err := scope.register()
			results <- registeredJob{scope: scope, record: record, err: err}
		}()
	}
	ready.Wait()

	// When
	close(start)
	admitted := collectRegisteredJobs(results, maxJobs*2)

	// Then
	if len(admitted) != maxJobs {
		t.Fatalf("admitted = %d, want %d", len(admitted), maxJobs)
	}
	consumeRegisteredJobs(t, admitted)
	root.cancel()
}

func TestJobSupervisor_failedAdmissionPreservesLocalID(t *testing.T) {
	// Given
	root, scopes := newJobScopeFamily(t, 1)
	private := scopes[1]
	first, err := private.register()
	if err != nil {
		t.Fatal(err)
	}
	rootRecords := registerJobs(t, root, maxJobs-1)

	// When
	_, rejectedErr := private.register()
	consumeJobs(t, root, rootRecords[:1])
	second, admittedErr := private.register()

	// Then
	if !errors.Is(rejectedErr, errJobLimit) || admittedErr != nil || first.id != 1 || second.id != 2 {
		t.Fatalf("rejected = %v, admitted = %v, IDs = %d,%d", rejectedErr, admittedErr, first.id, second.id)
	}
	consumeJobs(t, root, rootRecords[1:])
	consumeJobs(t, private, []*jobRecord{first, second})
	root.cancel()
}

func TestJobSupervisor_completionAndCancelledWaitRetainCapacityUntilOneRecordConsumed(t *testing.T) {
	// Given
	root, scopes := newJobScopeFamily(t, 1)
	private := scopes[1]
	records := registerJobs(t, root, maxJobs)
	root.complete(records[0], 7)
	claimed, ok := root.claim(records[0].id)
	if !ok {
		t.Fatal("claim failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, waitErr := waitJob(ctx, claimed)
	root.release(claimed)

	// When
	_, retainedErr := private.register()
	consumeJobs(t, root, records[:1])
	replacement, replacementErr := private.register()
	_, fullErr := private.register()

	// Then
	if waitErr == nil || !errors.Is(retainedErr, errJobLimit) || replacementErr != nil || !errors.Is(fullErr, errJobLimit) {
		t.Fatalf("wait = %v, retained = %v, replacement = %v, full = %v", waitErr, retainedErr, replacementErr, fullErr)
	}
	consumeJobs(t, root, records[1:])
	consumeJobs(t, private, []*jobRecord{replacement})
	root.cancel()
}

func TestJobSupervisor_consumeAndTeardownReleaseEachRecordExactlyOnce(t *testing.T) {
	// Given
	root, scopes := newJobScopeFamily(t, 2)
	tearingDown := scopes[1]
	sibling := scopes[2]
	records := registerJobs(t, tearingDown, maxJobs)
	for _, record := range records {
		tearingDown.complete(record, 0)
	}
	claimed, ok := tearingDown.claimAll()
	if !ok {
		t.Fatal("claim all failed")
	}
	start := make(chan struct{})
	done := make(chan struct{}, 2)

	// When
	go func() {
		<-start
		tearingDown.consumeAll(claimed)
		done <- struct{}{}
	}()
	go func() {
		<-start
		tearingDown.cancelAndDrain()
		done <- struct{}{}
	}()
	close(start)
	<-done
	<-done
	admitted := registerUntilRejected(t, sibling, maxJobs+1)

	// Then
	if len(admitted) != maxJobs {
		t.Fatalf("admitted after competing deletion = %d, want %d", len(admitted), maxJobs)
	}
	consumeJobs(t, sibling, admitted)
	root.cancel()
}

func TestJobSupervisor_duplicateConsumeIdentityReleasesOneSlot(t *testing.T) {
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

	// When
	consumed := scope.consumeAll([]*jobRecord{claimed, claimed})
	admitted := registerUntilRejected(t, scope, maxJobs+2)

	// Then
	if !consumed || len(admitted) != maxJobs {
		t.Fatalf("consumed = %t, admitted = %d, want true and %d", consumed, len(admitted), maxJobs)
	}
	consumeJobs(t, scope, admitted)
	scope.cancel()
}

func TestJobSupervisor_independentRootsHaveIndependentBudgets(t *testing.T) {
	// Given
	first := newRootJobScope()
	second := newRootJobScope()

	// When
	firstRecords := registerUntilRejected(t, first, maxJobs+1)
	secondRecords := registerUntilRejected(t, second, maxJobs+1)

	// Then
	if len(firstRecords) != maxJobs || len(secondRecords) != maxJobs {
		t.Fatalf("admitted = %d,%d, want independent limits of %d", len(firstRecords), len(secondRecords), maxJobs)
	}
	consumeJobs(t, first, firstRecords)
	consumeJobs(t, second, secondRecords)
	first.cancel()
	second.cancel()
}

func TestJobScope_privateScopeKeepsLocalIDsRecordsAndClaims(t *testing.T) {
	// Given
	root, scopes := newJobScopeFamily(t, 1)
	private := scopes[1]
	rootRecord, rootErr := root.register()
	privateRecord, privateErr := private.register()

	// When
	rootClaim, rootClaimed := root.claim(privateRecord.id)
	privateClaim, privateClaimed := private.claim(rootRecord.id)

	// Then
	rootRecords := root.snapshot()
	privateRecords := private.snapshot()
	if rootErr != nil || privateErr != nil || rootRecord.id != 1 || privateRecord.id != 1 || !rootClaimed || !privateClaimed {
		t.Fatalf("errors = %v,%v, IDs = %d,%d, claims = %t,%t", rootErr, privateErr, rootRecord.id, privateRecord.id, rootClaimed, privateClaimed)
	}
	if rootClaim != rootRecord || privateClaim != privateRecord || len(rootRecords) != 1 || rootRecords[0] != rootRecord || len(privateRecords) != 1 || privateRecords[0] != privateRecord {
		t.Fatalf("claims = %#v,%#v, root records = %#v, private records = %#v", rootClaim, privateClaim, rootRecords, privateRecords)
	}
	root.release(rootClaim)
	private.release(privateClaim)
	consumeJobs(t, root, []*jobRecord{rootRecord})
	consumeJobs(t, private, []*jobRecord{privateRecord})
	root.cancel()
}

type registeredJob struct {
	scope  *jobScope
	record *jobRecord
	err    error
}

func newJobScopeFamily(t *testing.T, privateCount int) (*jobScope, []*jobScope) {
	t.Helper()
	runtime := New(applets.DefaultRegistry, Streams{})
	scopes := []*jobScope{runtime.jobScope}
	for range privateCount {
		child, err := runtime.snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = child.fds.closeAll() })
		scopes = append(scopes, child.jobScope)
	}
	t.Cleanup(func() { _ = runtime.fds.closeAll() })
	return runtime.jobScope, scopes
}

func collectRegisteredJobs(results <-chan registeredJob, count int) []registeredJob {
	admitted := make([]registeredJob, 0, maxJobs)
	for range count {
		result := <-results
		if result.err == nil {
			admitted = append(admitted, result)
		}
	}
	return admitted
}

func consumeRegisteredJobs(t *testing.T, jobs []registeredJob) {
	t.Helper()
	for _, job := range jobs {
		consumeJobs(t, job.scope, []*jobRecord{job.record})
	}
}

func registerJobs(t *testing.T, scope *jobScope, count int) []*jobRecord {
	t.Helper()
	records := make([]*jobRecord, 0, count)
	for range count {
		record, err := scope.register()
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func registerUntilRejected(t *testing.T, scope *jobScope, attempts int) []*jobRecord {
	t.Helper()
	records := make([]*jobRecord, 0, attempts)
	for range attempts {
		record, err := scope.register()
		if errors.Is(err, errJobLimit) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func consumeJobs(t *testing.T, scope *jobScope, records []*jobRecord) {
	t.Helper()
	for _, record := range records {
		claimed, ok := scope.claim(record.id)
		if !ok {
			t.Fatalf("claim %d failed", record.id)
		}
		if !scope.consumeAll([]*jobRecord{claimed}) {
			t.Fatalf("consume %d failed", record.id)
		}
	}
}
