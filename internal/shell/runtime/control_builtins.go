package runtime

import (
	"context"
	"fmt"
)

// controlFlowBuiltin runs the builtins that answer with a control transfer
// rather than only a status. Each of them is a POSIX special builtin, so its
// leading assignments persist after it completes (2.9.1) and are applied here
// before the transfer leaves.
func (r Runtime) controlFlowBuiltin(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation, savedStatus int) (lineResult, bool) {
	switch args[0] {
	case "exit", "exec", "return", "break", "continue", "eval", ".", "source":
	default:
		return lineResult{}, false
	}
	if status := r.assignVars(assignments); status != 0 {
		return lineResult{status: status}, true
	}
	switch args[0] {
	case "eval":
		return r.withAppliedRedirects(operations, func(redirected Runtime) lineResult {
			return redirected.evalResult(ctx, args[1:])
		}), true
	case ".", "source":
		return r.withAppliedRedirects(operations, func(redirected Runtime) lineResult {
			return redirected.dotResult(ctx, args[1:])
		}), true
	case "exit":
		return lineResult{status: exitStatus(args[1:], savedStatus), control: flowExit}, true
	case "exec":
		if len(args) == 1 {
			return lineResult{status: r.execRedirect(operations)}, true
		}
		return lineResult{status: r.execBuiltin(ctx, args[1:]), control: flowExec}, true
	case "return":
		status := exitStatus(args[1:], savedStatus)
		if r.sourceDepth == 0 && r.functionDepth == 0 {
			fmt.Fprintln(r.streams.Stderr, "return: not in a sourced script")
			return lineResult{status: status}, true
		}
		return lineResult{status: status, control: flowReturn}, true
	case "break", "continue":
		return r.loopControlResult(args[0], args[1:]), true
	default:
		return lineResult{control: flowContinue}, true
	}
}
