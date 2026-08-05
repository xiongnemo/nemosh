package runtime

import "sync"

type jobSupervisor struct {
	mu    sync.Mutex
	count int
}

func (s *jobSupervisor) register(scopeSealed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count >= maxJobs {
		return errJobLimit
	}
	if scopeSealed {
		return errJobScopeSealed
	}
	s.count++
	return nil
}

func (s *jobSupervisor) release(count int) {
	s.mu.Lock()
	s.count -= count
	s.mu.Unlock()
}
