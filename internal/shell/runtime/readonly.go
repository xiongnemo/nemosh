package runtime

import (
	"fmt"
	"strings"
)

func (r Runtime) readonlyBuiltin(args []string) int {
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "" {
			return 2
		}
		if hasValue {
			if status := r.assignVar(name, value); status != 0 {
				return status
			}
		} else if _, ok := r.vars[name]; !ok {
			r.vars[name] = ""
		}
		r.readonly[name] = struct{}{}
	}
	return 0
}

func (r Runtime) assignVar(name string, value string) int {
	if r.isReadonly(name) {
		fmt.Fprintf(r.streams.Stderr, "%s: readonly variable\n", name)
		return 1
	}
	r.vars[name] = value
	return 0
}

func (r Runtime) isReadonly(name string) bool {
	_, ok := r.readonly[name]
	return ok
}
