package runtime

import (
	"context"
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

// assignVar is the one way a variable is written: readonly refused, RANDOM and SECONDS
// honoured, an element reached by its subscript, and the environment kept in step for a
// name that is exported or under `set -a`.
//
// **The one way, now.** The for loop, `select`, `${x:=word}` and arithmetic each wrote the
// map themselves, so each of them skipped all of that: `readonly R; : $((R=5))` changed R,
// and `export x=0; for x in 5; do env; done` showed a child `x=0`.
func (r Runtime) assignVar(name string, value string) int {
	if r.isReadonly(name) {
		return r.refuseReadonly("", name)
	}
	// RANDOM and SECONDS take an assignment as a seed and a reset rather than
	// storing it. Storing would make the next read return a constant, and a
	// `$RANDOM` that is always the same is the kind of thing noticed after the
	// damage. See special_vars.go.
	if r.assignSpecialVar(name, value) {
		return 0
	}
	// `a[0]=value`, where the value came from an expansion. The literal-value form
	// is settled before expansion by applyArrayAssignments; this is the same
	// destination reached from the other direction.
	if reference, ok := parseArrayReference(name); ok {
		// An element assignment reached through the string path. context.Background
		// rather than a threaded one: assignVar is called from thirty places, most
		// with no context, and the only thing the context reaches here is a key
		// held in a variable -- which needs no I/O. A command substitution in a
		// subscript is refused either way; see array_subscript.go.
		return r.assignElementByKind(context.Background(), reference, value)
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

// refuseReadonly reports an assignment to a readonly variable, and makes it a shell error.
//
// POSIX 2.8.1 makes a variable assignment error fatal to a non-interactive shell, and
// busybox-w32 holds to that everywhere measured: `R=2`, `R=2 cmd`, `export R=2`, `unset R`,
// `local R`, `for R in ...` and `$((R=5))` all end the script with status 2, and a
// subshell that does it ends only the subshell. `read R` is the exception there, and here
// -- it refuses, and the script goes on. This used to be a status of 1 and the next line.
func (r Runtime) refuseReadonly(prefix, name string) int {
	fmt.Fprintf(r.streams.Stderr, "%s%s: readonly variable\n", prefix, name)
	r.raiseShellError()
	return 1
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
