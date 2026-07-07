package runtime

import (
	"fmt"
	"strconv"
)

type parameters struct {
	values []string
}

func (r Runtime) set(args []string) int {
	if len(args) == 2 && args[0] == "-o" && args[1] == "pipefail" {
		r.options.pipefail = true
		return 0
	}
	if len(args) > 0 && args[0] == "--" {
		r.params.values = append(r.params.values[:0], args[1:]...)
	}
	return 0
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
