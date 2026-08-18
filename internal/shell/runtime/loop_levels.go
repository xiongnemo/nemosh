package runtime

import (
	"fmt"
	"strconv"
)

// `break n` and `continue n`: leave or resume the nth enclosing loop.
//
// The operand used to be parsed and then thrown away, so `break 2` broke one loop
// and the outer one kept going. Like `read -r`, that is not a missing feature but a
// wrong answer with no diagnostic: the script runs, does something else, and the
// author has no reason to look here.
//
// The count rides on shared state rather than in the flowControl value, which is
// what keeps the change small. flowControl is returned through a dozen
// `(int, flowControl)` signatures, and threading a level through all of them would
// touch every compound statement to express something only loops read. Instead the
// level is set when `break` runs and consumed by each loop it passes through --
// the same arrangement childCPU and history already use for state that is the
// shell's rather than any one snapshot's.
type loopLevels struct {
	// remaining is how many more loops the pending break or continue still has to
	// escape. Zero when nothing is pending, which is the ordinary case.
	remaining int
	// depth is how many loops are currently running, so a count larger than the
	// nesting can be clamped instead of escaping the nest. See request.
	depth int
}

func newLoopLevels() *loopLevels { return &loopLevels{} }

// enter and leave bracket a running loop.
func (l *loopLevels) enter() { l.depth++ }

func (l *loopLevels) leave() {
	if l.depth > 0 {
		l.depth--
	}
}

// request records a `break n` or `continue n`, clamped to the loops that actually
// exist.
//
// The clamp is not tidiness. `for a in 1 2; do break 5; done; echo out` printed
// nothing without it: the four unused levels carried the break past the loop and
// out through the rest of the script, so `echo out` never ran. bash treats the
// count as "at most this many" and stops at the outermost loop.
func (l *loopLevels) request(count int) {
	if l.depth > 0 && count > l.depth {
		count = l.depth
	}
	l.remaining = count
}

// consume is called by each loop the control passes through, and reports whether
// this loop is the one meant.
//
// A count larger than the nesting depth is not an error: bash breaks every
// enclosing loop and stops, so the counter is cleared when it runs out of loops
// rather than left pending for the next one.
func (l *loopLevels) consume() bool {
	if l.remaining <= 1 {
		l.remaining = 0
		return true
	}
	l.remaining--
	return false
}

// clear drops a pending level. A loop that finishes normally must not leave a
// count behind for an unrelated loop later in the script.
func (l *loopLevels) clear() { l.remaining = 0 }

// parseLoopLevel reads the operand of `break` or `continue`.
//
// Measured against bash and dash: a missing operand is 1, a non-numeric one is a
// diagnostic and a failure, and 0 is refused -- `break 0` would have to mean
// "break no loops", which is not something either shell will do quietly.
func parseLoopLevel(word string, name string) (int, error) {
	if word == "" {
		return 1, nil
	}
	count, err := strconv.Atoi(word)
	if err != nil {
		return 0, fmt.Errorf("%s: %s: numeric argument required", name, word)
	}
	if count < 1 {
		return 0, fmt.Errorf("%s: %s: loop count out of range", name, word)
	}
	return count, nil
}

// loopControlResult turns `break`/`continue` and their operand into the result the
// dispatcher returns.
func (r Runtime) loopControlResult(name string, args []string) lineResult {
	operand := ""
	if len(args) > 0 {
		operand = args[0]
	}
	count, err := parseLoopLevel(operand, name)
	if err != nil {
		fmt.Fprintln(r.streams.Stderr, err)
		// bash exits 128 for a bad count on a special builtin in POSIX mode and 1
		// otherwise; 1 is what an interactive shell shows and what a script can
		// act on, so the flow is left alone and only the status reports.
		return lineResult{status: 1}
	}
	// Outside any loop this does nothing and the script carries on. Measured:
	// busybox ash, dash and bash all continue and all exit 0, bash after printing
	// a note. Silence follows the primary reference. Before the depth was tracked
	// there was nothing to test, and the break escaped to the top of the script
	// instead -- `break; echo after` printed nothing at all.
	if r.loops.depth == 0 {
		return lineResult{status: 0}
	}
	r.loops.request(count)
	if name == "break" {
		return lineResult{control: flowBreak}
	}
	return lineResult{control: flowContinue}
}
