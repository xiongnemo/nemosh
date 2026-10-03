package runtime

import (
	"fmt"
	"strconv"
)

type parameters struct {
	name   string
	values []string
	// function is the function these parameters were passed to, which is what
	// `$FUNCNAME` answers; empty outside one. Kept here because a call already
	// makes a new set of parameters and restores the caller's on the way out.
	function string
	// getopts is where getopts is in these parameters, so replacing them starts it again;
	// see getopts.go.
	getopts getoptsState
}

// SetArguments seeds $0 and the positional parameters. POSIX gives the name and
// the arguments separate lives: set -- and a function call replace $1... while
// $0 keeps naming the script for the whole run.
func (r Runtime) SetArguments(name string, positional []string) {
	r.params.name = name
	r.params.values = append(r.params.values[:0], positional...)
	r.params.getopts = getoptsState{}
}

func (r Runtime) shift(args []string) int {
	count := 1
	if len(args) > 0 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil || parsed < 0 {
			// It ends a script, as busybox's number() raises it, and is said as the ash family
			// says it: shift is a special builtin.
			fmt.Fprintf(r.streams.Stderr, "%sshift: Illegal number: %s\n", r.diagnosticPrefix(), args[0])
			r.raiseShellError()
			return 2
		}
		count = parsed
	}
	if count > len(r.params.values) {
		// Both references are silent here; bash says so under `shopt -s shift_verbose`, in
		// these words.
		if r.options.shiftVerbose && len(args) > 0 {
			fmt.Fprintf(r.streams.Stderr, "%sshift: %s: shift count out of range\n", r.diagnosticPrefix(), args[0])
		} else if r.options.shiftVerbose {
			fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"shift: shift count out of range")
		}
		return 1
	}
	r.params.values = r.params.values[count:]
	r.params.getopts = getoptsState{}
	return 0
}
