package runtime

import (
	"context"
	"errors"
	"sort"
	"sync"
)

const maxJobs = 64

var errJobLimit = errors.New("job limit reached")
var errJobScopeSealed = errors.New("job scope sealed")

type jobID uint64

type jobRecord struct {
	id      jobID
	done    chan struct{}
	status  int
	claimed bool
}

type jobScope struct {
	mu         sync.Mutex
	next       jobID
	records    map[jobID]*jobRecord
	ctx        context.Context
	cancel     context.CancelFunc
	supervisor *jobSupervisor
	sealed     bool
}

func newRootJobScope() *jobScope {
	ctx, cancel := context.WithCancel(context.Background())
	return &jobScope{records: make(map[jobID]*jobRecord), ctx: ctx, cancel: cancel, supervisor: &jobSupervisor{}}
}

func newPrivateJobScope(parent context.Context, supervisor *jobSupervisor) *jobScope {
	ctx, cancel := context.WithCancel(parent)
	return &jobScope{records: make(map[jobID]*jobRecord), ctx: ctx, cancel: cancel, supervisor: supervisor}
}

func (s *jobScope) register() (*jobRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.supervisor.register(s.sealed)
	if errors.Is(err, errJobLimit) {
		// A slot is released by `wait`, and a script need never call it -- `foo &`
		// on its own is ordinary -- so without this a session that has started
		// maxJobs background jobs refuses every one after them, permanently,
		// however long ago they finished. Measured, busybox starts its hundred
		// and first without complaint. The limit is meant to bound jobs that are
		// still running, so the finished ones are swept when the space is needed.
		if s.reapFinishedLocked() > 0 {
			err = s.supervisor.register(s.sealed)
		}
	}
	if err != nil {
		return nil, err
	}
	s.next++
	record := &jobRecord{id: s.next, done: make(chan struct{})}
	s.records[record.id] = record
	return record, nil
}

// reapFinishedLocked drops records whose job has completed and which nothing is
// waiting on, and reports how many were dropped. The caller holds s.mu.
//
// Sweeping only under pressure rather than on every registration is deliberate:
// below the limit a finished job stays visible to `jobs` as Done and addressable
// by `wait %N`, which is what busybox shows. A claimed record is left alone
// because a `wait` is already holding it.
func (s *jobScope) reapFinishedLocked() int {
	deleted := 0
	for id, record := range s.records {
		if record.claimed {
			continue
		}
		select {
		case <-record.done:
			delete(s.records, id)
			deleted++
		default:
		}
	}
	if deleted > 0 {
		s.supervisor.release(deleted)
	}
	return deleted
}

func (s *jobScope) complete(record *jobRecord, status int) {
	s.mu.Lock()
	record.status = status
	close(record.done)
	s.mu.Unlock()
}

func (s *jobScope) snapshot() []*jobRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]*jobRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].id < records[j].id })
	return records
}

func (s *jobScope) claim(id jobID) (*jobRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok || record.claimed {
		return nil, false
	}
	record.claimed = true
	return record, true
}

func (s *jobScope) claimAll() ([]*jobRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]*jobRecord, 0, len(s.records))
	for _, record := range s.records {
		if record.claimed {
			return nil, false
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].id < records[j].id })
	for _, record := range records {
		record.claimed = true
	}
	return records, true
}

func (s *jobScope) release(record *jobRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.records[record.id]; ok && current == record {
		record.claimed = false
	}
}

func (s *jobScope) releaseAll(records []*jobRecord) {
	for _, record := range records {
		s.release(record)
	}
}

func (s *jobScope) consumeAll(records []*jobRecord) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		if current, ok := s.records[record.id]; !ok || current != record || !record.claimed {
			return false
		}
	}
	deleted := 0
	for _, record := range records {
		if s.records[record.id] == record {
			delete(s.records, record.id)
			deleted++
		}
	}
	s.supervisor.release(deleted)
	return true
}

func waitJob(ctx context.Context, record *jobRecord) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	select {
	case <-record.done:
		return record.status, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func waitJobs(ctx context.Context, records []*jobRecord) ([]int, error) {
	statuses := make([]int, len(records))
	for index, record := range records {
		status, err := waitJob(ctx, record)
		if err != nil {
			return nil, err
		}
		statuses[index] = status
	}
	return statuses, nil
}

func (s *jobScope) drain() {
	records, ok := s.claimAll()
	if !ok {
		return
	}
	_, _ = waitJobs(s.ctx, records)
	s.consumeAll(records)
}

func (s *jobScope) cancelAndDrain() {
	s.mu.Lock()
	s.sealed = true
	records := make([]*jobRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	s.mu.Unlock()
	s.cancel()
	_, _ = waitJobs(context.Background(), records)
	s.mu.Lock()
	deleted := 0
	for _, record := range records {
		if s.records[record.id] == record {
			delete(s.records, record.id)
			deleted++
		}
	}
	s.supervisor.release(deleted)
	s.mu.Unlock()
}

func (s *jobScope) seal() {
	s.mu.Lock()
	s.sealed = true
	s.mu.Unlock()
}
