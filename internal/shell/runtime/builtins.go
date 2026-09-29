package runtime

import (
	"context"
	"fmt"
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
	return r.dotResult(ctx, args, 0, false).status
}

// dotResult runs `. FILE`, args[0] the name it was called by; special is whether it runs as the
// special builtin, which a `command` prefix takes away (POSIX 2.14).
func (r Runtime) dotResult(ctx context.Context, args []string, savedStatus int, special bool) lineResult {
	name, args := args[0], args[1:]
	// `--` before the file, which busybox's `.` and bash's source both take; it was read as
	// the file's name.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s: missing file\n", name)
		return lineResult{status: 2}
	}
	// A file that cannot be read is busybox's `cannot open FILE: no such file`, and a shell error:
	// it ends a script with 2, as POSIX 2.8.1 has it and busybox's dotcmd does, and abandons the
	// line at a prompt. `command .` runs it as a plain builtin, which goes on with the 2. It went
	// on with status 1, as bash does outside its POSIX mode, and the error named the host path.
	native, device, err := r.dotSource(args[0])
	var data []byte
	if err == nil {
		data, err = r.readDotSource(native, device)
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", name, redirectFailure{path: args[0], err: err})
		if !special {
			return lineResult{status: 2}
		}
		return lineResult{status: 2, control: flowAbort}
	}
	child := r.enterFrame("source", args[0])
	child.sourceDepth++
	return child.withDotArguments(args[1:], func() lineResult {
		debug, debugSet := child.hideDebugTrap()
		defer child.restoreDebugTrap(debug, debugSet)
		status, control := child.runScriptResult(ctx, string(data), 1, false, savedStatus)
		if _, set := r.traps[trapRETURN]; set && (control == flowNone || control == flowReturn) {
			child.runTrap(ctx, trapRETURN, status)
		}
		if control == flowReturn {
			return lineResult{status: status}
		}
		return lineResult{status: status, control: control}
	})
}

func (r Runtime) eval(ctx context.Context, args []string) int {
	return r.evalResult(ctx, args, 0).status
}

func (r Runtime) evalResult(ctx context.Context, args []string, savedStatus int) lineResult {
	if len(args) == 0 {
		return lineResult{}
	}
	status, control := r.runScriptResult(ctx, strings.Join(args, " "), r.currentLine(), false, savedStatus)
	return lineResult{status: status, control: control}
}
