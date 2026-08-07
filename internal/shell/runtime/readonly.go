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
		r.markVarMutation(name)
	}
	return 0
}

func (r Runtime) assignVar(name string, value string) int {
	if r.isReadonly(name) {
		fmt.Fprintf(r.streams.Stderr, "%s: readonly variable\n", name)
		return 1
	}
	r.vars[name] = value
	// `set -a` exports every name an assignment touches, so a variable set
	// after it is on reaches children without a separate `export`.
	if _, exported := r.env.LookupEnv(name); exported || r.allExport() {
		r.env.Set(name, value)
	}
	r.markVarMutation(name)
	return 0
}

// allExport is nil-safe: a Runtime built by hand for a focused test carries
// only the fields that test is about, which is how markVarMutation guards its
// own map too.
func (r Runtime) allExport() bool {
	return r.options != nil && r.options.allExport
}

func (r Runtime) isReadonly(name string) bool {
	_, ok := r.readonly[name]
	return ok
}

func (r Runtime) markVarMutation(name string) {
	if r.mutatedVars != nil {
		r.mutatedVars[name] = struct{}{}
	}
}
