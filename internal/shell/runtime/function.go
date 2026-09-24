package runtime

import (
	"context"
	"fmt"
)

const maxFunctionCallDepth = 128

// functionCommand runs a command that names a function, and answers with what the body
// ended with -- `exit`, `break`, a shell error -- rather than only its status.
//
// **`exit` in a function did not exit.** `die() { echo "$1" >&2; exit 1; }` is one of the
// commonest things in shell, and `die boom; echo next` printed `next` and went on with status
// 0: the call answered an int, so the exit the body produced stopped at the function and
// went no further. It had been that way since the typed runtime, and nothing tested it. Only
// `return` stops at a function, which is the one control a call consumes; every other one
// carries on out, as it does in both references. busybox lets a `break` in a function leave
// the caller's loop, and so does this.
//
// A special builtin is found before a function, as runCommandResolved has it, so this
// declines one. Prefix assignments are the function's for the call and restored after it --
// both references restore `x` after `x=2 f` even when f assigned x itself -- and whatever
// else the body changed stays changed.
func (r Runtime) functionCommand(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation) (lineResult, bool) {
	if len(args) == 0 || isSpecialBuiltin(args[0]) {
		return lineResult{}, false
	}
	name, ok := newFunctionName(args[0])
	if !ok {
		return lineResult{}, false
	}
	definition, found := r.functions[name]
	if !found {
		return lineResult{}, false
	}
	caller := r
	var temporary *Runtime
	if len(assignments) > 0 {
		if temporary = r.withLocalAssignments(assignments); temporary == nil {
			return lineResult{status: 1}, true
		}
		caller = *temporary
	}
	result := caller.withAppliedRedirects(operations, func(redirected Runtime) lineResult {
		return redirected.callFunctionResult(ctx, definition, args[1:])
	})
	if temporary != nil {
		for _, assignment := range assignments {
			delete(temporary.mutatedVars, assignment.name)
		}
		r.mergeBuiltinMutations(*temporary)
	}
	if ctx.Err() != nil && result.control == flowNone {
		result.status = contextStatus(ctx)
	}
	return result, true
}

func (r Runtime) callFunction(ctx context.Context, definition functionDefinition, args []string) int {
	return r.callFunctionResult(ctx, definition, args).status
}

func (r Runtime) callFunctionResult(ctx context.Context, definition functionDefinition, args []string) lineResult {
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call: %v\n", err)
		return lineResult{status: 1}
	}
	if r.functionDepth >= maxFunctionCallDepth {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call depth exceeds %d\n", maxFunctionCallDepth)
		return lineResult{status: 1}
	}
	r = r.enterFrame(definition.name.value, definition.file)
	r.params = &parameters{name: r.params.name, values: append([]string(nil), args...), function: definition.name.value}
	r.functionDepth++
	// A call gets its own local scope, and whatever `local` shadowed inside it
	// is put back on the way out -- including when the body returns early or
	// breaks out of a loop, which is why the restore is deferred.
	scope := newLocalScope()
	r.locals = scope
	defer scope.restore(r)
	hidden, wasSet := r.hideReturnTrap()
	result := r.executeCommandNode(ctx, definition.body, 0)
	r.finishReturnTrap(ctx, hidden, wasSet, result)
	if result.control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	if result.control == flowReturn {
		return lineResult{status: result.status}
	}
	return result
}
