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
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, false, value.line)
	case subshellCommand:
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, true, value.line)
	default:
		return lineResult{status: 2}
	}
}

func (r Runtime) executeCompoundCommand(ctx context.Context, body Script, redirects []redirectOperation, savedStatus int, isolated bool, closer int) lineResult {
	commandRuntime := r
	if isolated {
		var err error
		commandRuntime, err = r.subshellSnapshotTracing(ctx, r.traceTurn.subshell())
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
			return lineResult{status: 1}
		}
		commandRuntime.seeNoLoopsAsBash()
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
	if line := r.compoundRedirectLine(body, redirects, closer); line > 0 {
		commandRuntime.enterLine(line)
	}
	ran := false
	result := commandRuntime.executeWithRedirects(ctx, redirects, savedStatus, func(redirected Runtime) lineResult {
		ran = true
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
	// A group's redirections that failed, a loop's, an if's or a case's -- a compound with them
	// is a group around it -- run the ERR trap and end the script under set -e in bash mode, as
	// bash 5.2 and later have it; a lone group is passed over where a pipeline's failure is
	// answered, since its commands answered for themselves. busybox goes on, as nemosh and sh do.
	// A subshell's are answered with the pipeline it is in.
	if !ran && !isolated && r.BashMode() && result.control == flowNone && result.status != 0 {
		if r.errTrapTriggers(result) {
			r.runTrap(ctx, trapERR, result.status)
		}
		if r.errExitTriggers(result) {
			result.control = flowExit
		}
	}
	return result
}

// compoundRedirectLine is the line a compound command's redirection is said on when it fails:
// the line the command begins on, as busybox-w32 says it. It was said on the line of the command
// run before it. bash says the line the redirection is written on, which for a compound written
// over several lines, `while ...` and then `done < nofile`, is its last, the closer's line the
// parser kept, and bash mode says that. 0 when there is no redirection, or no line to give.
func (r Runtime) compoundRedirectLine(body Script, redirects []redirectOperation, closer int) int {
	if len(redirects) == 0 || len(body.program) == 0 {
		return 0
	}
	if closer > 0 && r.BashMode() {
		return closer
	}
	return statementLine(body.program[0])
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
