package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// dot and eval run text as part of the current shell, so whatever that text does to the
// shell's flow it does to the caller's: `exit` in a sourced file exits the shell, `break`
// in an eval leaves the loop around it, and a shell error aborts as it would have written
// out in full. Only `return` stops at a sourced file, which is the one thing POSIX says
// `.` consumes.
//
// **They used to keep the status and drop the rest.** `. ./lib.sh` whose lib called `exit
// 4` carried on at the next line with status 0; `eval "exit 3"` printed what came after it;
// `for i in 1 2 3; do eval break; done` ran all three times. busybox does none of that.
// The results carry the control now, and controlFlowBuiltin is where they are dispatched,
// beside the other builtins whose answer is more than a status.

func (r Runtime) dot(ctx context.Context, args []string) int {
	return r.dotResult(ctx, args).status
}

func (r Runtime) dotResult(ctx context.Context, args []string) lineResult {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, ".: missing file")
		return lineResult{status: 2}
	}
	resolved, err := r.ResolveNemoshPath(args[0])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, ".: %s: %v\n", args[0], err)
		return lineResult{status: 1}
	}
	if resolved.Device {
		fmt.Fprintf(r.streams.Stderr, ".: %s: not a regular file\n", args[0])
		return lineResult{status: 1}
	}
	data, err := os.ReadFile(resolved.Native)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, ".: %s: %v\n", args[0], err)
		return lineResult{status: 1}
	}
	child := r.enterFrame("source", args[0])
	child.sourceDepth++
	status, control := child.runScriptResult(ctx, string(data), 1, false)
	if _, set := r.traps[trapRETURN]; set && (control == flowNone || control == flowReturn) {
		child.runTrap(ctx, trapRETURN, status)
	}
	if control == flowReturn {
		return lineResult{status: status}
	}
	return lineResult{status: status, control: control}
}

func (r Runtime) eval(ctx context.Context, args []string) int {
	return r.evalResult(ctx, args).status
}

func (r Runtime) evalResult(ctx context.Context, args []string) lineResult {
	if len(args) == 0 {
		return lineResult{}
	}
	status, control := r.runScriptResult(ctx, strings.Join(args, " "), r.currentLine(), false)
	return lineResult{status: status, control: control}
}
