package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// A signal `kill` sends a background job, step three of docs/design/background-processes.md.
// The job's trap for it runs when the command in progress has finished. That is when bash
// runs one: `(trap 'echo got' TERM; sleep 1; echo after) & kill $!` prints got, then after.
// A `trap ''` makes the job ignore it. Anything else ends the job, and its EXIT trap still
// runs; the status is 128+n. busybox-w32 cannot run a trap for a signal at all, since
// every kill there ends the target, so here bash is the reference.
//
// The inbox holds a signal until the job reaches a boundary, which the job's own goroutine
// does. The sender is another goroutine, and the job's trap table is not safe to read from
// there, so the inbox keeps its own copy of the one thing the sender needs to know: whether
// the signal has a trap or is ignored. When it has neither, offer refuses it, because the
// default action is the sender's to take.

// signalTraps are the signals a trap can be set for, by number: the ones `kill` sends, less
// KILL, which nothing can catch.
var signalTraps = map[int]trapName{1: trapHUP, 2: trapINT, 3: trapQUIT, 15: trapTERM}

// signalNumber is the number of the signal a trap name is for, or 0.
func signalNumber(name trapName) int {
	for number, trap := range signalTraps {
		if trap == name {
			return number
		}
	}
	return 0
}

type signalInbox struct {
	mu      sync.Mutex
	caught  map[int]bool
	pending []int
	waiting atomic.Bool
}

func newSignalInbox() *signalInbox {
	return &signalInbox{caught: map[int]bool{}}
}

// offer holds signal for the next boundary if a trap catches it or an empty one ignores
// it, and reports whether it did. A nil inbox belongs to a subshell, which a signal is never addressed to.
func (b *signalInbox) offer(signal int) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.caught[signal] {
		return false
	}
	b.pending = append(b.pending, signal)
	b.waiting.Store(true)
	return true
}

// catch records whether name's signal is caught, as `trap` arms or resets it.
func (b *signalInbox) catch(name trapName, caught bool) {
	signal := signalNumber(name)
	if b == nil || signal == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if caught {
		b.caught[signal] = true
	} else {
		delete(b.caught, signal)
	}
}

func (b *signalInbox) take() []int {
	if b == nil || !b.waiting.Load() {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	pending := b.pending
	b.pending = nil
	b.waiting.Store(false)
	return pending
}

// setTrap arms or resets a trap. An action of "-" resets it.
func (r Runtime) setTrap(name trapName, action string) {
	if action == "-" {
		delete(r.traps, name)
	} else {
		r.traps[name] = action
	}
	r.signals.catch(name, action != "-")
}

// deliverSignals runs the traps of the signals that arrived during the command that has
// just finished. The command's status is what `$?` is inside each trap and after them all,
// unless a trap ends the shell: `trap 'exit 3' TERM` is how a job says it was stopped.
func (r Runtime) deliverSignals(ctx context.Context, result lineResult) lineResult {
	for _, signal := range r.signals.take() {
		name := signalTraps[signal]
		action, set := r.traps[name]
		if !set {
			// Reset between the send and now, so the default after all.
			return lineResult{status: 128 + signal, control: flowExit}
		}
		if action == "" {
			continue
		}
		trapped := r.runTrap(ctx, name, result.status)
		if trapped.control != flowNone {
			return trapped
		}
	}
	return result
}

// jobSignal is the cause a job's context ends with when a signal it does not catch ends it.
type jobSignal int

func (s jobSignal) Error() string { return fmt.Sprintf("signal %d", int(s)) }
