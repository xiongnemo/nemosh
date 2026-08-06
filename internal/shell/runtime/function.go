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
	result := r.executeCommandNode(ctx, definition.body, 0)
	if result.control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	if result.control == flowReturn {
		return result.status
	}
	return result.status
}
