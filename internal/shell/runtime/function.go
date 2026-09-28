package runtime

import (
	"context"
	"fmt"
	"strings"
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
// A function is found before any builtin of its name, a special one too, as busybox-w32 and
// bash both look a command up: `exit() { cleanup; command exit "$@"; }` wraps exit. The
// special builtins, and before them the ones that change the flow of control, were found
// first, POSIX's order, so such a function was defined and never called. `command` still
// skips it. Prefix assignments are the function's for the call and restored after it -- both
// references restore `x` after `x=2 f` even when f assigned x itself -- and whatever else the
// body changed stays changed.
func (r Runtime) functionCommand(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation, savedStatus int) (lineResult, bool) {
	if len(args) == 0 {
		return lineResult{}, false
	}
	definition, found := r.calledFunction(args[0])
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
		return redirected.callFunctionResult(ctx, definition, args[1:], savedStatus)
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
	return r.callFunctionResult(ctx, definition, args, 0).status
}

// callFunctionResult runs the body with `$?` as the caller had it, as both references do, so
// a bare `return` first thing returns it and an ERR trap's handler sees the status that fired
// it. It started at 0, and `err() { echo $?; }; trap err ERR` said 0 for every failure.
func (r Runtime) callFunctionResult(ctx context.Context, definition functionDefinition, args []string, savedStatus int) lineResult {
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call: %v\n", err)
		return lineResult{status: 1}
	}
	if r.functionDepth >= maxFunctionCallDepth {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call depth exceeds %d\n", maxFunctionCallDepth)
		return lineResult{status: 1}
	}
	// Once the call returns, `$LINENO` is the line that made it again, as both references
	// have it, so an ERR trap its failure fires in the caller names the caller's line. It
	// named the function's last one.
	defer r.enterLine(r.currentLine())
	caller := r.params
	r = r.enterFrame(definition.name.value, definition.file)
	r.params = &parameters{name: r.params.name, values: append([]string(nil), args...), function: definition.name.value}
	r.functionDepth++
	// A call gets its own local scope, and whatever `local` shadowed inside it
	// is put back on the way out -- including when the body returns early or
	// breaks out of a loop, which is why the restore is deferred. Put back beside the
	// caller's parameters, so a local OPTIND restored is where the caller's getopts starts
	// over, as busybox has it.
	scope := newLocalScope()
	r.locals = scope
	restoring := r
	restoring.params = caller
	defer scope.restore(restoring)
	hidden, wasSet := r.hideReturnTrap()
	result := r.executeCommandNode(ctx, definition.body, savedStatus)
	r.finishReturnTrap(ctx, hidden, wasSet, result)
	if result.control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	if result.control == flowReturn {
		return lineResult{status: result.status}
	}
	return result
}

// calledFunction is the function a command word calls, if one is defined by that name. A
// word with a slash is a path, and calls no function though one may be defined by it --
// busybox-w32's answer; bash calls the function.
func (r Runtime) calledFunction(word string) (functionDefinition, bool) {
	if strings.Contains(word, "/") {
		return functionDefinition{}, false
	}
	name, ok := newFunctionName(word)
	if !ok {
		return functionDefinition{}, false
	}
	definition, found := r.functions[name]
	return definition, found
}
