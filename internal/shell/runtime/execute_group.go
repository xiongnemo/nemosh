package runtime

import (
	"context"
	"errors"
	"fmt"
)

func (r Runtime) executeCommandNode(ctx context.Context, command commandNode, savedStatus int) lineResult {
	// A command after `set -n` on the same line, or in the same group; see
	// readsWithoutExecuting.
	if r.readsWithoutExecuting() {
		return lineResult{control: flowExec}
	}
	switch value := command.(type) {
	case simpleCommand:
		return r.executeSimpleCommand(ctx, value, savedStatus)
	case braceGroup:
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, false)
	case subshellCommand:
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, true)
	default:
		return lineResult{status: 2}
	}
}

func (r Runtime) executeCompoundCommand(ctx context.Context, body Script, redirects []redirectOperation, savedStatus int, isolated bool) lineResult {
	commandRuntime := r
	if isolated {
		var err error
		commandRuntime, err = r.subshellSnapshotTracing(ctx, r.traceTurn.subshell())
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
			return lineResult{status: 1}
		}
		// A subshell's job table is its own, and starts empty: `( jobs )` lists nothing in
		// both references. See listedJobs.
		commandRuntime.jobScope.outer = nil
		defer func() {
			commandRuntime.jobScope.cancelAndDrain()
			_ = commandRuntime.fds.closeAll()
		}()
	}
	return commandRuntime.executeWithRedirects(ctx, redirects, savedStatus, func(redirected Runtime) lineResult {
		// A subshell is a shell of its own, which a write into a pipe no one reads ends, and
		// the shell goes on; see pipe_stage.go.
		bodyCtx, release := ctx, func() {}
		if isolated {
			bodyCtx, redirected, release = redirected.ownStage(ctx, false)
		}
		defer release()
		status, control := redirected.executeProgram(bodyCtx, body.program, savedStatus)
		if isolated {
			// Not after an `exec`, which replaced the subshell: nothing is left to
			// run its trap, in either reference.
			if control != flowExec {
				status = redirected.runOwnExitTrap(ctx, r.traps[trapExit], status)
			}
			control = flowNone
		}
		return lineResult{status: status, control: control}
	})
}

func (r Runtime) executeWithRedirects(ctx context.Context, operations []redirectOperation, savedStatus int, run func(Runtime) lineResult) lineResult {
	operations, ok := r.expandRedirectOperations(ctx, cloneRedirects(operations), savedStatus)
	if !ok {
		return lineResult{status: 1}
	}
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		r.reportFailedRedirection(errors.Join(err, table.closeAll()))
		return lineResult{status: 1}
	}
	result := run(r.withFDTable(table))
	if err := table.closeAll(); err != nil && result.status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		result.status = 1
	}
	return result
}
