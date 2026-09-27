package runtime

import "sync"

type jobSupervisor struct {
	mu    sync.Mutex
	count int
	// handoffs are the job states still being written, each closed when its write ends; see
	// handOff.
	handoffs []chan struct{}
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

// handOff runs write, which gives a job process its state, on a goroutine: `cmd &` does not
// wait for the job to start and read it. The shell's exit does wait, in awaitHandoffs. A
// shell that exited first took the write with it, and the job read nothing and ended --
// `nemosh -c 'sleep 3 &'` said "job state: unexpected end of JSON input" and never slept.
func (s *jobSupervisor) handOff(write func()) {
	done := make(chan struct{})
	s.mu.Lock()
	pending := s.handoffs[:0]
	for _, handoff := range s.handoffs {
		select {
		case <-handoff:
		default:
			pending = append(pending, handoff)
		}
	}
	s.handoffs = append(pending, done)
	s.mu.Unlock()
	go func() {
		defer close(done)
		write()
	}()
}

// awaitHandoffs waits for every job state still being written. Each write ends once its job
// has read the state or has ended, so this is as long as a job takes to start.
func (s *jobSupervisor) awaitHandoffs() {
	s.mu.Lock()
	pending := s.handoffs
	s.handoffs = nil
	s.mu.Unlock()
	for _, handoff := range pending {
		<-handoff
	}
}
