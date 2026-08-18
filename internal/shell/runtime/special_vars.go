package runtime

import (
	"math/rand/v2"
	"os"
	"strconv"
	"time"
)

// The variables the shell computes rather than stores: $RANDOM, $SECONDS, $PPID,
// and the $PIPESTATUS array.
//
// All four were simply unset, which reads as the empty string, so `$RANDOM` in a
// script that wanted a temporary name produced the same name every time and
// `${PIPESTATUS[0]}` never reported anything about a pipeline. Unset is not a lie
// the way `read -r` was -- an unset variable *is* empty -- but it is the wrong
// answer to a question these names exist to answer.
//
// $LINENO is deliberately not here. It needs the line each command came from, and
// nothing in the AST carries one: programNode has no position, so plumbing it would
// mean touching the parser and every node. Left out rather than faked, because a
// $LINENO that is always 1 is worse than one that is absent -- it would send
// somebody to the wrong line with confidence.

// specialState holds what the computed variables need to be computed from.
//
// Shared by pointer like the rest of the shell's own state, so a subshell keeps
// counting from the same start rather than resetting $SECONDS to zero.
type specialState struct {
	// started is when the shell began, for $SECONDS.
	started time.Time
	// secondsOffset is what `SECONDS=n` set, so the count resumes from n rather
	// than from zero. bash allows the assignment and this follows it.
	secondsOffset int
	// random is seeded per shell rather than per read, so a script gets a
	// sequence rather than the same number twice.
	random *rand.Rand
}

func newSpecialState() *specialState {
	return &specialState{
		started: time.Now(),
		// Seeded from the clock and the pid: two shells started in the same
		// millisecond should not agree, which is exactly the case a script using
		// $RANDOM for a temporary name runs into.
		random: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(os.Getpid()))),
	}
}

// randomMaximum is bash's range for $RANDOM: 0 through 32767 inclusive.
const randomMaximum = 32768

// dynamicParameter answers the computed variables, and reports whether the name is
// one of them.
//
// Consulted after the ordinary variables, so a script that sets `RANDOM=` to
// disable something of its own still sees what it set. The one exception is that
// assigning to RANDOM or SECONDS is taken as a seed or a reset rather than stored;
// see assignSpecialVar.
func (r Runtime) dynamicParameter(name string) (string, bool) {
	if r.special == nil {
		return "", false
	}
	switch name {
	case "RANDOM":
		return strconv.Itoa(r.special.random.IntN(randomMaximum)), true
	case "SECONDS":
		elapsed := int(time.Since(r.special.started).Seconds())
		return strconv.Itoa(elapsed + r.special.secondsOffset), true
	case "PPID":
		return strconv.Itoa(os.Getppid()), true
	}
	return "", false
}

// assignSpecialVar intercepts a write to one of the computed names, and reports
// whether it handled it.
//
// RANDOM reseeds and SECONDS resets, which is what bash does; storing the value
// instead would make the next read return a constant, and a `$RANDOM` that is
// always 4 is the kind of thing that gets noticed after the damage. PPID is
// read-only in bash and is refused here, through the ordinary readonly path so the
// diagnostic is the same one every other read-only name gets.
func (r Runtime) assignSpecialVar(name, value string) bool {
	if r.special == nil {
		return false
	}
	switch name {
	case "RANDOM":
		seed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return false
		}
		r.special.random = rand.New(rand.NewPCG(seed, seed))
		return true
	case "SECONDS":
		offset, err := strconv.Atoi(value)
		if err != nil {
			return false
		}
		r.special.started, r.special.secondsOffset = time.Now(), offset
		return true
	}
	return false
}

// recordPipeStatus fills $PIPESTATUS, which is the only way to find out that the
// first stage of `false | true` failed -- `$?` reports the last stage and always
// will.
//
// Set for a single command too, as a one-element array: bash does, and a script
// that reads `${PIPESTATUS[0]}` after a plain command should not get the leftovers
// of the pipeline before it.
func (r Runtime) recordPipeStatus(statuses ...int) {
	if r.arrays == nil {
		return
	}
	elements := make([]string, 0, len(statuses))
	for _, status := range statuses {
		elements = append(elements, strconv.Itoa(status))
	}
	r.arrays.set("PIPESTATUS", elements)
}
