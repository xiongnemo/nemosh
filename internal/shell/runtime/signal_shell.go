package runtime

import (
	"context"
	"errors"
	"sync"
)

// The shell's own signals, from outside it. Off Windows that is a real `kill`. On
// Windows it is the console closing, the user logging off or the machine shutting down,
// which Go reports as SIGTERM. A script gets them as a job does (signal_inbox.go): a
// signal's trap runs when the command in progress finishes, and a signal nothing catches
// ends the script, whose EXIT trap still runs, with 128+n. bash 5.3 answers the same for
// a script it runs: `trap 'echo bye' EXIT; sleep 5` sent TERM says bye and exits 143.

// ReceiveSignals watches signals, as numbers, while a script runs. It answers with the
// context to run the script under, which a signal nothing catches ends, and the function
// that stops the watch. After stop, ExitSignal still says whether a signal ended it.
//
// final is a signal that ends the script whatever catches it, and its trap then runs as
// the script ends, before the EXIT trap. A console closing is that: Windows sends it to
// every process on the console, the command in progress too, and ends them all within
// seconds. Waiting for the command to finish, as bash's rule has it, would be waiting
// past the end: `trap ... TERM; sleep 20` never ran its trap when the window closed.
func (r Runtime) ReceiveSignals(ctx context.Context, signals <-chan int, final bool) (context.Context, func()) {
	ctx, end := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case signal := <-signals:
				if final || !r.signals.offer(signal) {
					end(jobSignal(signal))
				}
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			close(done)
			end(context.Canceled)
		})
	}
}

// ExitSignal is the signal that ended ctx, if one did.
func ExitSignal(ctx context.Context) (int, bool) {
	signal, ok := errors.AsType[jobSignal](context.Cause(ctx))
	return int(signal), ok
}

// ExitBySignal ends this process by signal, in the form a shell that started it reads
// as one -- `Terminated`, not `Done(143)`. It does not return.
func ExitBySignal(signal int) {
	endBySignal(signal)
}

// EndJobs ends every background job the shell still has running, their programs with
// them, and returns once they have ended. cmd/nemosh calls it when Ctrl-C has ended a
// script, after the script's own traps have had their say. busybox-w32 ends a script's
// jobs that way, measured by pressing Ctrl-C in its console, and so did this shell while
// its jobs were goroutines, which died with the process. A job that is a process would
// otherwise run on, often a loop nobody can see any more. bash leaves them running,
// since POSIX has an asynchronous list ignore SIGINT; busybox's answer is the one kept.
func (r Runtime) EndJobs() {
	r.jobScope.cancelAndDrain()
}
