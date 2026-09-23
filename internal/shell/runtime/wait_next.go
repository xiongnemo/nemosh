package runtime

import (
	"context"
	"reflect"
	"sort"
)

// waitNext is `wait -n`: wait for whichever job ends first, and answer its status.
//
// busybox has it and bash has it, and this shell used to refuse it as a second operand it
// did not expect -- which a script did not notice, because the plain `wait` that usually
// follows waited for everything anyway. What it is for is a pool: start four jobs, and start
// another each time one of them finishes.
//
// With operands, only those jobs are candidates. With nothing left to wait for, 127, which
// both references answer. A job that has already ended counts as the next one, lowest first,
// so a finished job is never passed over for one still running.
//
// The status is the job's own, as bash answers; busybox-w32 answers 0 here even for a job
// that exited 3, which reads as a fault rather than a choice.
func (r Runtime) waitNext(ctx context.Context, operands []string) int {
	records, status := r.nextCandidates(operands)
	if status != 0 {
		return status
	}
	if len(records) == 0 {
		return 127
	}
	index, err := firstEnded(ctx, records)
	if err != nil {
		r.jobScope.releaseAll(records)
		return contextStatus(ctx)
	}
	ended := records[index]
	for _, record := range records {
		if record != ended {
			r.jobScope.release(record)
		}
	}
	r.reportSignalled([]*jobRecord{ended})
	if !r.jobScope.consumeAll([]*jobRecord{ended}) {
		return 2
	}
	return ended.status
}

// nextCandidates claims the jobs `wait -n` may answer with: the ones named, or every job no
// other `wait` is holding. An operand the shell does not know is said so and passed over, so
// that `wait -n %1 %2` still waits for whichever of the two exists.
func (r Runtime) nextCandidates(operands []string) ([]*jobRecord, int) {
	if len(operands) == 0 {
		return r.jobScope.claimUnclaimed(), 0
	}
	var records []*jobRecord
	for _, operand := range operands {
		id, status := r.waitTarget(operand)
		if status == 2 {
			r.jobScope.releaseAll(records)
			return nil, 2
		}
		if status != 0 {
			continue
		}
		if record, claimed := r.jobScope.claim(id); claimed {
			records = append(records, record)
			continue
		}
		r.unclaimable(operand, id)
	}
	return records, 0
}

// claimUnclaimed claims every job that no `wait` is holding, in id order.
func (s *jobScope) claimUnclaimed() []*jobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []*jobRecord
	for _, record := range s.records {
		if !record.claimed {
			record.claimed = true
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].id < records[j].id })
	return records
}

// firstEnded answers which of these jobs ends first, or the context's error.
//
// One that has already ended is answered at once, lowest id first, rather than left to a
// select that would choose among several ready ones at random.
func firstEnded(ctx context.Context, records []*jobRecord) (int, error) {
	for index, record := range records {
		select {
		case <-record.done:
			return index, nil
		default:
		}
	}
	cases := make([]reflect.SelectCase, 0, len(records)+1)
	for _, record := range records {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(record.done)})
	}
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())})
	chosen, _, _ := reflect.Select(cases)
	if chosen == len(records) {
		return 0, ctx.Err()
	}
	return chosen, nil
}
