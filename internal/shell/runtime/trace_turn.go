package runtime

import (
	"sync"
	"time"
)

// traceTurn orders the `set -x` lines of a pipeline's stages as busybox and bash write them.
// There each stage is a process started after the one before it, and a stage goes on writing
// lines until it starts a program or forks, by which time the next is under way. So `set -x;
// echo a | cat | tr a b` traces echo, cat and tr in that order on every run; `g | cat` traces
// g and the lines of g's body before cat; and `x=$(echo sub) | cat` traces cat before echo
// sub, the substitution being one fork further from the shell than cat is. Here each stage is
// a goroutine, and the lines came in whichever order the goroutines reached them -- the last
// stage's first, as often as not.
//
// So a stage's lines wait for its turn, which it has once the stage before has passed on its
// own. It passes it on when it starts a program, forks, writes more into its pipe than the
// pipe is sure to hold, would wait to read its pipe, or ends; and the lines of what it forks
// wait for the pipeline's last stage to have passed its turn. However the turn is spent, it
// passes a moment after the stage's first line, or a second after it came if no line has been
// written: a line can be late, but nothing waits on one for ever.
type traceTurn struct {
	// after is closed once the stage before has passed its turn; nil for the first stage.
	after <-chan struct{}
	// gate is closed once the pipeline's lines may begin. For a pipeline a stage runs, or
	// anything it forks, that is once the outer pipeline's last stage has passed its turn.
	gate <-chan struct{}
	// passed is closed once the turn has passed on, requested once the stage asks for that,
	// and lined once it has written a line.
	passed, requested, lined chan struct{}
	request, line            sync.Once
	// last is the turn of the pipeline's last stage, and next the next stage's.
	last, next *traceTurn
	// child is the turn of what a stage forked, whose lines wait for gate alone and which has
	// no turn to pass on.
	child bool
}

const (
	// traceTurnHold is how long a stage keeps its turn after its first line: longer than a
	// run of builtins takes, and short enough that one waiting on input holds no one up long.
	traceTurnHold = 100 * time.Millisecond
	// traceTurnWait is how long a stage keeps its turn before any line.
	traceTurnWait = time.Second
	// traceTurnPipeBytes is what a stage may write into its pipe and keep its turn: less than
	// any system's pipe holds, so no write waits for a reader that is waiting for its turn.
	traceTurnPipeBytes = 1024
)

// newTraceTurns makes the turns of a pipeline's stages, each after the one before it. A
// pipeline run inside a stage is something the stage forks.
func newTraceTurns(count int, outer *traceTurn) []*traceTurn {
	gate := outer.forkGate()
	turns := make([]*traceTurn, count)
	var after <-chan struct{}
	for index := range turns {
		turns[index] = &traceTurn{after: after, gate: gate, passed: make(chan struct{}), requested: make(chan struct{}), lined: make(chan struct{})}
		after = turns[index].passed
	}
	for index, turn := range turns {
		turn.last = turns[count-1]
		if index+1 < count {
			turn.next = turns[index+1]
		}
		go turn.hold()
	}
	return turns
}

// hold keeps the turn from when it is the stage's until it is passed on.
func (t *traceTurn) hold() {
	defer close(t.passed)
	t.ready()
	came := time.NewTimer(traceTurnWait)
	defer came.Stop()
	select {
	case <-t.requested:
		return
	case <-came.C:
		return
	case <-t.lined:
	}
	held := time.NewTimer(traceTurnHold)
	defer held.Stop()
	select {
	case <-t.requested:
	case <-held.C:
	}
}

// ready waits for the turn to be the stage's.
func (t *traceTurn) ready() {
	if t.after != nil {
		<-t.after
	}
	if t.gate != nil {
		<-t.gate
	}
}

// write writes a trace line once the stage has its turn. A line after the turn has passed on
// waits for the next stage to have begun: the program the stage started, or what it forked, is
// a process to start in either reference, and the next stage's first line comes before it has.
func (t *traceTurn) write(line func()) {
	if t == nil {
		line()
		return
	}
	t.ready()
	if t.next != nil && !t.holding() {
		t.next.begun()
	}
	line()
	if !t.child {
		t.line.Do(func() { close(t.lined) })
	}
}

// passOn gives the next stage its turn, once this one has had it.
func (t *traceTurn) passOn() {
	if t != nil && !t.child {
		t.request.Do(func() { close(t.requested) })
	}
}

// holding answers whether the stage has yet to pass its turn on, or to ask to.
func (t *traceTurn) holding() bool {
	select {
	case <-t.requested:
		return false
	case <-t.passed:
		return false
	default:
		return true
	}
}

// begun waits for the stage to have written a line or passed its turn on.
func (t *traceTurn) begun() {
	select {
	case <-t.lined:
	case <-t.passed:
	}
}

// forkGate is what the lines of something the stage forks wait for: the pipeline's last stage
// to have passed its turn, after this stage has passed its own.
func (t *traceTurn) forkGate() <-chan struct{} {
	switch {
	case t == nil:
		return nil
	case t.child:
		return t.gate
	}
	t.passOn()
	return t.last.passed
}

// fork is the turn of a substitution, a subshell or a job the stage starts.
func (t *traceTurn) fork() *traceTurn {
	if t == nil || t.child {
		return t
	}
	return &traceTurn{gate: t.forkGate(), child: true}
}

// subshell is the turn of a `( )`: the stage's own when the stage has begun with it, for the
// stage's process is then the subshell, as `echo a | (cat) | tr a b` traces cat before tr in
// both references; a fork's once the stage has written a line or passed its turn.
func (t *traceTurn) subshell() *traceTurn {
	if t == nil || t.child || !t.holding() {
		return t.fork()
	}
	select {
	case <-t.lined:
		return t.fork()
	default:
		return t
	}
}

// startingProgram is the stage starting an applet or a program, by when the next stage in
// either reference is under way. ash's own builtins among the applets start nothing.
func (r Runtime) startingProgram(name string) {
	if !ashBuiltinApplets[name] {
		r.traceTurn.passOn()
	}
}
