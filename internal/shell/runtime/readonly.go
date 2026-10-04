package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) readonlyBuiltin(ctx context.Context, args []string) int {
	options, args, ok := r.readonlyOptions(args)
	switch {
	case !ok:
		return 2
	case len(args) == 0:
		return r.listReadonly()
	case options.indexed || options.associative:
		return r.readonlyArrays(ctx, options, args)
	}
	for _, arg := range args {
		target, value, hasValue := cutAssignment(arg)
		name, appended := splitAssignmentTarget(target)
		if !isValidVariableName(name) {
			return r.refuseName("readonly: ", name)
		}
		// An array literal makes a read-only array, as bash's `declare -ar` would: `readonly
		// r=(r e)` held the text (r e). See arrayLiteralOperands.
		if hasValue && r.arrayOperands[arg] {
			if status := r.declareName(ctx, declareOptions{readonly: true}, arg); status != 0 {
				return status
			}
			continue
		}
		// A name with no value stays unset, read-only from now on: `readonly X` made X
		// empty, where both references leave `${X-unset}` saying unset.
		if hasValue {
			if appended {
				value = r.appendedValue(name, value)
			}
			if status := r.assignVar(name, value); status != 0 {
				return status
			}
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
	// Through a nameref, to what it leads to; see nameref.go.
	target, handled, err := r.namerefAssignment(name, value)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1
	}
	if handled {
		return 0
	}
	if name, err = r.namerefElement(target); err != nil {
		// A subscript on a nameref to an element, `ref[0]=x`, is no identifier: the statement
		// is abandoned, status 1, as bash abandons it. It went on with 0.
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		r.failAssignment()
		return 1
	}
	if r.isReadonly(name) {
		return r.refuseReadonly("", name)
	}
	// `declare -i -l -u`, here so every way a value arrives is treated alike.
	value, err = r.applyAttributes(name, value)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%s: %v\n", r.diagnosticPrefix(), name, err)
		r.failAssignment()
		return 1
	}
	// On Windows a list of directories takes `:` as well as `;`; see path_list.go.
	value = windowsPathList(name, value)
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
	// An array's bare name is its element zero, to write as to read: `a=(p q); a=z` is
	// `z q` in bash. This wrote only the scalar mirror, so `$a` said z while `${a[@]}`
	// still said `p q`.
	if r.isArrayName(name) {
		return r.assignElementByKind(context.Background(), arrayReference{name: name, subscript: "0"}, value)
	}
	r.vars[name] = value
	// `set -a` exports every name an assignment touches, so a variable set
	// after it is on reaches children without a separate `export`.
	if r.isExported(name) || r.allExport() {
		r.env.Set(name, value)
	}
	r.markVarMutation(name)
	return 0
}

// isArrayName reports a name that is an array of either kind. Nil-safe for the same reason
// allExport is.
func (r Runtime) isArrayName(name string) bool {
	if r.arrays == nil {
		return false
	}
	indexed := r.arrays.has(name)
	return indexed || r.arrays.isAssociative(name)
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
	fmt.Fprintf(r.streams.Stderr, "%s%s%s: is read only\n", r.diagnosticPrefix(), prefix, name)
	r.raiseShellError()
	return 1
}

// refuseName reports an operand that is not a variable name, in busybox's words, and makes
// it a shell error, which ends a script as it does there. `export a/b` returned 0 -- and the
// tail of a value split at a space, `export PATH=$PATH:...` over `C:/Program Files`, went
// quietly into the environment as a name nothing could read back.
func (r Runtime) refuseName(prefix, name string) int {
	fmt.Fprintf(r.streams.Stderr, "%s%s%s: bad variable name\n", r.diagnosticPrefix(), prefix, name)
	r.raiseShellError()
	return 2
}

// isReadonly answers for a name, or for the array an element belongs to: `readonly a`
// protects `a[0]` as well.
func (r Runtime) isReadonly(name string) bool {
	if reference, ok := parseArrayReference(name); ok {
		name = reference.name
	}
	_, ok := r.readonly[name]
	return ok
}

func (r Runtime) markVarMutation(name string) {
	if r.mutatedVars != nil {
		r.mutatedVars[name] = struct{}{}
	}
	// OPTIND assigned, unset or made local is where getopts starts over; see getoptsState.
	if name == "OPTIND" && r.params != nil {
		r.params.getopts = getoptsFrom(r.vars["OPTIND"])
	}
	// HISTSIZE assigned trims the list at once, the newest kept, as bash's sv_histsize
	// does; it waited for the next line to be recorded.
	if name == "HISTSIZE" {
		r.history.truncate(r.historyLimit())
	}
}
