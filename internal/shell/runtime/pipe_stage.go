package runtime

import (
	"context"
	"sync/atomic"
	"syscall"
	"time"
)

// A shell and SIGPIPE.
//
// A write into a pipe whose reader has gone kills the writer with SIGPIPE, and which writer
// it is decides what dies. Each of the shell, a subshell and a pipeline stage is a process in
// both references, and each is one here too: a write of its own, a builtin's output, ends it
// there with 141, and nothing after the write runs. A program it runs dies alone, with 141,
// and it goes on: `{ cat big; echo after >&2; } | head -1` still says after, and so does
// `(while :; do echo x; done); echo after >&2` into `head -1`. Windows has no SIGPIPE, and
// none of the three is a process here, so the write only failed, quietly, and a loop that
// wrote into `head` wrote on for ever.

// pipeStage is a shell as SIGPIPE sees it: the shell, a subshell or a pipeline stage.
type pipeStage struct {
	// programs is how many programs the shell is running now. A failed write while one runs
	// is the program's, and ends only it.
	programs atomic.Int32
	abandon  context.CancelCauseFunc
}

// brokenPipeStatus is what a write into a pipe no one reads leaves: SIGPIPE's 128+13.
const brokenPipeStatus = 128 + int(syscall.SIGPIPE)

// readerGone ends the shell as SIGPIPE ends it, unless the write was a program's.
func (s *pipeStage) readerGone() {
	if s != nil && s.programs.Load() == 0 {
		s.abandon(jobSignal(syscall.SIGPIPE))
	}
}

// running counts a program the shell runs, until the func it returns is called.
func (s *pipeStage) running() func() {
	if s == nil {
		return func() {}
	}
	s.programs.Add(1)
	return func() { s.programs.Add(-1) }
}

// ownStage makes r a shell of its own until release: its context, which a write of its own
// into a standard output no one reads ends, and the runtime to run it with. Its standard
// output answers to it, and its standard error with it when `2>&1` made them one file; one
// the stages of a pipeline share is left to the shell. top is the shell itself, whose
// standard error is its own.
func (r Runtime) ownStage(ctx context.Context, top bool) (context.Context, Runtime, func()) {
	ctx, abandon := context.WithCancelCause(ctx)
	stage := &pipeStage{abandon: abandon}
	r.pipeStage = stage
	restores := r.fds.answerTo(stage, top)
	return ctx, r, func() {
		for _, restore := range restores {
			restore()
		}
		abandon(nil)
	}
}

// abandonedStatus is the status of a shell that ended as SIGPIPE ends it, while what it runs
// in goes on.
func abandonedStatus(outer, own context.Context, status int) int {
	if outer.Err() == nil && own.Err() != nil {
		return contextStatus(own)
	}
	return status
}

// answerTo points standard output, and standard error as ownStage says, at stage, and
// answers with what puts each back.
func (t *fdTable) answerTo(stage *pipeStage, top bool) []func() {
	output, err := t.lookup(1)
	if err != nil {
		return nil
	}
	descriptions := []*openDescription{output.description}
	if standardError, err := t.lookup(2); err == nil && top && standardError.description != output.description {
		descriptions = append(descriptions, standardError.description)
	}
	restores := make([]func(), 0, len(descriptions))
	for _, description := range descriptions {
		previous := description.stage.Swap(stage)
		restores = append(restores, func() { description.stage.Store(previous) })
	}
	return restores
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
