package runtime

import (
	"context"
	"sync/atomic"
	"syscall"
	"time"
)

// A pipeline stage, as SIGPIPE sees it.
//
// A write into a pipe whose reader has gone kills the writer with SIGPIPE, and which writer
// it is decides what dies. A program the stage runs dies alone, with 141: in both references
// `{ cat big; echo after >&2; } | head -1` still says after. The shell running the stage dies
// when it is the writer, as a builtin's output makes it: `while :; do echo x; done | head -2`
// ends there, the stage with 141, and nothing after the write runs. Windows has no SIGPIPE,
// and a stage here is a goroutine rather than a process, so the write only failed, quietly,
// and that loop wrote on for ever.

// pipeStage is the stage of a pipeline whose pipe a runtime's output goes into.
type pipeStage struct {
	// programs is how many programs the stage is running now. A failed write while one runs
	// is the program's, and ends only it.
	programs atomic.Int32
	abandon  context.CancelCauseFunc
}

// brokenPipeStatus is what a write into a pipe no one reads leaves: SIGPIPE's 128+13.
const brokenPipeStatus = 128 + int(syscall.SIGPIPE)

// readerGone ends the stage as SIGPIPE ends a shell, unless the write was a program's.
func (s *pipeStage) readerGone() {
	if s != nil && s.programs.Load() == 0 {
		s.abandon(jobSignal(syscall.SIGPIPE))
	}
}

// unreadGrace is how long the read end of a pipe stays open after the stage reading it has
// ended without reading any of it, while the stage writing into it goes on.
const unreadGrace = 50 * time.Millisecond

// lingers is whether closing this end waits: it is a read end nothing was read from. A stage
// that reads nothing -- `| true`, `| false` -- can end before the one feeding it has written a
// byte, which a reader that is a process to start and to end seldom does. Closed at once, the
// pipe took that first write for one no one reads: `{ echo hi; exit 55; } | false` said 141
// where both references say 55. It closes once the writer is done or the grace is up, and a
// writer that is still writing finds it closed then.
func (e *pipelineEndpoint) lingers() bool {
	return e.peer != nil && !e.consumed.Load()
}

func (e *pipelineEndpoint) closeLater() {
	select {
	case <-e.peer.closed:
	case <-time.After(unreadGrace):
	}
	e.closeNow()
}

// running counts a program the stage runs, until the func it returns is called.
func (s *pipeStage) running() func() {
	if s == nil {
		return func() {}
	}
	s.programs.Add(1)
	return func() { s.programs.Add(-1) }
}
