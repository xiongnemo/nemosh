package runtime

import (
	"fmt"
	"sync/atomic"
)

// A trap on SIGPIPE, which pipe_stage.go models. `trap "" PIPE` is how a script that pipes
// into head asks for a failed write rather than an ended shell, and it was "not supported by
// this shell", status 1, which ends a script under `set -e` at that line. busybox-w32 takes it.
//
// A shell that ignores SIGPIPE does not end when a write of its own finds no reader: the
// write fails and the builtin says so, `echo: write error: Broken pipe` and status 1, as
// bash's does and as ash_test echo_write_error has it. A program it runs sees its failed
// write too, as a program that inherited the ignored signal does. A shell that catches
// SIGPIPE has the same failed write, and runs the trap once the command in progress has
// finished; a program still dies of it, since a caught signal is the default again in a new
// program. A subshell keeps an ignored SIGPIPE and drops a trap for it, as POSIX has it.

// trapPIPE is SIGPIPE's trap.
const trapPIPE trapName = "PIPE"

// pipeDisposition is what SIGPIPE does to a shell.
type pipeDisposition int32

const (
	pipeDefault pipeDisposition = iota
	pipeIgnored
	pipeCaught
)

// dispositionOf is what a trap action makes SIGPIPE: `-` the default, an empty one ignored,
// anything else caught.
func dispositionOf(action string) pipeDisposition {
	switch action {
	case "-":
		return pipeDefault
	case "":
		return pipeIgnored
	}
	return pipeCaught
}

// pipeDispositionFor is SIGPIPE's disposition in a shell that starts with r's traps, the
// shell itself or a subshell, which keeps it ignored and drops a trap for it.
func (r Runtime) pipeDispositionFor(top bool) pipeDisposition {
	action, set := r.traps[trapPIPE]
	switch {
	case !set:
		return pipeDefault
	case action == "":
		return pipeIgnored
	case top:
		return pipeCaught
	}
	return pipeDefault
}

// dispose makes disposition what SIGPIPE does to the shell, as `trap` arms or resets it, and
// to the process when the shell is the process's own.
func (s *pipeStage) dispose(disposition pipeDisposition) {
	if s == nil {
		return
	}
	s.disposition.Store(int32(disposition))
	if s.top && processSIGPIPEOwned.Load() {
		setProcessPipe(disposition)
	}
}

func (s *pipeStage) current() pipeDisposition {
	if s == nil {
		return pipeDefault
	}
	return pipeDisposition(s.disposition.Load())
}

// takeCaught reports whether a write of the shell's own found no reader while a trap caught
// SIGPIPE, since it last asked.
func (s *pipeStage) takeCaught() bool {
	return s != nil && s.caught.Swap(false)
}

// brokenPipe is the status of a builtin or a program whose write found no reader: SIGPIPE's,
// or 1 with the write error said, when the shell ignores SIGPIPE, or catches it and the write
// was a builtin's, the shell's own.
func (r Runtime) brokenPipe(name string) int {
	disposition := r.pipeStage.current()
	if disposition == pipeDefault || disposition == pipeCaught && !ashBuiltinApplets[name] {
		return brokenPipeStatus
	}
	fmt.Fprintf(r.streams.Stderr, "%s: write error: Broken pipe\n", name)
	return 1
}

// processSIGPIPEOwned is set by a binary whose shell's SIGPIPE is the process's: cmd/nemosh,
// which runs one shell. A test's many shells share its process, and none of them may set it.
var processSIGPIPEOwned atomic.Bool

// OwnProcessSIGPIPE says the shells this binary starts own the process's SIGPIPE, as a shell's
// is its process's: `trap "" PIPE` at the top makes the process ignore it, and the programs it
// starts inherit that. cmd/nemosh calls it before anything else.
func OwnProcessSIGPIPE() { processSIGPIPEOwned.Store(true) }
