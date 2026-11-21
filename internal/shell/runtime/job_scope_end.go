package runtime

// letRun is a subshell ending: no job is started after it, one just started is given its
// state, and the jobs still running run on, as a subshell's do in both references -- `(sleep
// 9 &)` returns at once and the sleep goes on. They leave its table, which nothing lists
// again, and the count. Its context, which a job is under, ends once the last of them has: it
// has to end, or the parent's would hold it for ever.
func (s *jobScope) letRun() {
	s.seal()
	s.mu.Lock()
	var running []*jobRecord
	taken := 0
	for id, record := range s.records {
		delete(s.records, id)
		taken++
		select {
		case <-record.done:
		default:
			running = append(running, record)
		}
	}
	s.supervisor.release(taken)
	s.mu.Unlock()
	if len(running) == 0 {
		s.cancel()
		return
	}
	go func() {
		for _, record := range running {
			<-record.done
		}
		s.cancel()
	}()
}
