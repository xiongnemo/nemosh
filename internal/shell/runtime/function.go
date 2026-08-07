package runtime

import (
	"context"
	"fmt"
)

const maxFunctionCallDepth = 128

func (r Runtime) callFunction(ctx context.Context, definition functionDefinition, args []string) int {
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call: %v\n", err)
		return 1
	}
	if r.functionDepth >= maxFunctionCallDepth {
		fmt.Fprintf(r.streams.Stderr, "nemosh: function call depth exceeds %d\n", maxFunctionCallDepth)
		return 1
	}
	r.params = &parameters{name: r.params.name, values: append([]string(nil), args...)}
	r.functionDepth++
	// A call gets its own local scope, and whatever `local` shadowed inside it
	// is put back on the way out -- including when the body returns early or
	// breaks out of a loop, which is why the restore is deferred.
	scope := newLocalScope()
	r.locals = scope
	defer scope.restore(r.vars)
	result := r.executeCommandNode(ctx, definition.body, 0)
	if result.control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	if result.control == flowReturn {
		return result.status
	}
	return result.status
}
