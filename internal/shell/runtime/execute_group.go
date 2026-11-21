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
			fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
			return lineResult{status: 1}
		}
		// A subshell's job table is its own, and starts empty: `( jobs )` lists nothing in
		// both references. See listedJobs. It is under the shell's job scope rather than this
		// command's context, which ends with the run, since a job it starts runs on once it
		// has ended (letRun below).
		commandRuntime.jobScope.cancel()
		commandRuntime.jobScope = newPrivateJobScope(r.jobScope.ctx, r.jobScope.supervisor)
		defer func() {
			// A job it started runs on once it has ended, as in both references; an
			// interrupt that ended it ends them too, busybox's answer to a Ctrl-C.
			if ctx.Err() != nil {
				commandRuntime.jobScope.cancelAndDrain()
			} else {
				commandRuntime.jobScope.letRun()
			}
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
	// `done < <(cmd)`: the file is the command's input until it is done and the table that
	// holds it closed; see heldSubstitutions.
	defer r.releaseSubstitutions(r.expansion.held())
	operations, ok := r.expandRedirectOperations(ctx, cloneRedirects(operations), savedStatus)
	// A word that failed to expand, `> ${x?oops}`, is answered as its error is, as a simple
	// command's is, and nothing is opened: the empty path it left was, and said so.
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	if !ok {
		return lineResult{status: 1}
	}
	// In the shell's own descriptors when there are no redirections, as withAppliedRedirectsFor
	// says: `{ exec 3>log; }` keeps fd 3, as busybox-w32 does.
	if len(operations) == 0 {
		return run(r)
	}
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 1}
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		r.reportFailedRedirection(errors.Join(err, table.closeAll()))
		return lineResult{status: 1}
	}
	result := run(r.withFDTable(table))
	if err := table.closeAll(); err != nil && result.status == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		result.status = 1
	}
	return result
}
