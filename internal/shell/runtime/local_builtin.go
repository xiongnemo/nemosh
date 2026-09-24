package runtime

import (
	"context"
	"fmt"
	"strings"
)

// local declares variables that belong to the function call that runs it, so
// the value they had outside comes back when the call returns.
//
// It is not in POSIX -- 2.9.5 makes every variable a function touches global --
// but ash, dash and bash all have it and nearly every real script leans on it,
// so a shell without it silently leaks working variables into its caller.
// busybox carries it as BUILTIN_SPEC_REG_ASSG (shell/ash.c:12101).
//
// It takes declare's options, as bash's does: `local -a list=(...)`, `local -A map`,
// `local -i n`, `local -r`. Each was `bad variable name`, and the call went on with the
// value unset or unevaluated -- `local -i n=2+3` held the text 2+3. It is `declare` inside
// a function, which is also what `declare` itself now is there (declareInFunction).
//
// Outside a function there is nothing to restore to, and the shells report that
// rather than quietly behaving like an assignment.
func (r Runtime) local(ctx context.Context, args []string) int {
	if r.functionDepth == 0 || r.locals == nil {
		fmt.Fprintln(r.streams.Stderr, "local: not in a function")
		return 1
	}
	options, names, err := parseDeclareOptions(args)
	if err == nil && (options.print || options.functionNames || options.global) {
		err = fmt.Errorf("-p, -F and -g are declare's; local declares")
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "local: %v\n", err)
		return 2
	}
	for _, arg := range names {
		if arg == "-" {
			r.locals.saveOptions(r.options)
			continue
		}
		target, _, _ := strings.Cut(arg, "=")
		if name, _ := splitAssignmentTarget(target); !isVariableName(name) {
			return r.refuseName("local: ", name)
		}
		if status := r.declareLocal(ctx, "local", options, arg); status != 0 {
			return status
		}
	}
	return 0
}

// declareLocal makes one `name` or `name=value` the running call's own and then declares
// it. A read-only name cannot be shadowed, and that refusal is fatal as an assignment to
// it is.
func (r Runtime) declareLocal(ctx context.Context, builtin string, options declareOptions, arg string) int {
	target, _, _ := strings.Cut(arg, "=")
	name, _ := splitAssignmentTarget(target)
	if _, already := r.locals.saved[name]; !already && r.isReadonly(name) {
		return r.refuseReadonly(builtin+": ", name)
	}
	r.makeLocal(name)
	return r.declareName(ctx, options, arg)
}
