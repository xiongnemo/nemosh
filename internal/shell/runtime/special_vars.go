package runtime

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"
)

// The variables the shell computes rather than stores: $RANDOM, $SECONDS, $PPID,
// $FUNCNAME, $LINENO, $EPOCHSECONDS and $EPOCHREALTIME, and the $PIPESTATUS array.
//
// All four were simply unset, which reads as the empty string, so `$RANDOM` in a
// script that wanted a temporary name produced the same name every time and
// `${PIPESTATUS[0]}` never reported anything about a pipeline. Unset is not a lie
// the way `read -r` was -- an unset variable *is* empty -- but it is the wrong
// answer to a question these names exist to answer.
//
// $LINENO is the line the running command starts on. Each command carries its line
// from the parser (line_numbers.go); it was unset, so `set -u` stopped any script that
// named it -- and naming it is what an error handler does.

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
	case "FUNCNAME":
		// The function running now, which is busybox's `$FUNCNAME` and the first
		// element of bash's array; empty, and so unset, outside one. It used to be
		// empty everywhere, with no error, so a `log()` that named its caller named
		// nothing.
		if r.params != nil && r.params.function != "" {
			return r.params.function, true
		}
		return "", false
	case "LINENO":
		return strconv.Itoa(r.currentLine()), true
	case "BASH_SOURCE", "BASH_LINENO":
		// The first element, as a bare array name is; see call_stack.go.
		if stack, _ := r.callStackArray(name); len(stack) > 0 {
			return stack[0], true
		}
		return "", false
	case "EPOCHSECONDS":
		// Both references have these two, and a timestamp without forking `date` is
		// what they are for. EPOCHREALTIME has six decimal places, as both give it.
		return strconv.FormatInt(time.Now().Unix(), 10), true
	case "EPOCHREALTIME":
		now := time.Now()
		return fmt.Sprintf("%d.%06d", now.Unix(), now.Nanosecond()/1000), true
	case "$":
		// `$$`, the shell's own process id. Answered here rather than in either
		// expansion switch, because there are two of them -- the braced path and the
		// bare one -- and a special parameter that only one knows about is how `$$`
		// came to work inside `${...}` and not on its own.
		return strconv.Itoa(os.Getpid()), true
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
