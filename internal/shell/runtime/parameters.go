package runtime

import (
	"fmt"
	"strconv"
)

type parameters struct {
	name   string
	values []string
}

// SetArguments seeds $0 and the positional parameters. POSIX gives the name and
// the arguments separate lives: set -- and a function call replace $1... while
// $0 keeps naming the script for the whole run.
func (r Runtime) SetArguments(name string, positional []string) {
	r.params.name = name
	r.params.values = append(r.params.values[:0], positional...)
}

func (r Runtime) shift(args []string) int {
	count := 1
	if len(args) > 0 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil || parsed < 0 {
			fmt.Fprintf(r.streams.Stderr, "shift: invalid count: %s\n", args[0])
			return 2
		}
		count = parsed
	}
	if count > len(r.params.values) {
		return 1
	}
	r.params.values = r.params.values[count:]
	return 0
}
