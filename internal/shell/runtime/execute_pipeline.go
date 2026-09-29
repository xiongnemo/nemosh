package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) executeTypedPipeline(ctx context.Context, value pipeline, savedStatus int) lineResult {
	// Everything inside `!` is exempt from `set -e` and the ERR trap, not only the pipeline's
	// own status, as in both references: `set -e; ! { false; }` ended the script at the false.
	stages := r
	if value.negated {
		stages = r.suppressingErrExit()
	}
	result, aliased := stages.runLoneAlias(ctx, value, savedStatus)
	if !aliased {
		result = stages.executeTypedPipelineStages(ctx, value, savedStatus)
	}
	// POSIX 2.9.2: `!` gives the logical NOT of the pipeline's exit status. A
	// control transfer is not a status, so `! exit 3` still exits with 3.
	if value.negated && result.control == flowNone {
		if result.status == 0 {
			result.status = 1
		} else {
			result.status = 0
		}
	}
	// POSIX 2.9.1 exempts a negated pipeline from `set -e`, along with the
	// places the caller marks by suppressing it. The ERR trap runs in exactly
	// the same places, and first, so a handler set with `set -e` still runs.
	// A lone brace group is not a second turn: its status is its last command's,
	// which had its own, so `set -e; { false && true; }` goes on as both
	// references go on, and `{ false; }` fires the trap once. A subshell still
	// fails in the shell that ran it. An alias's commands have had theirs too, as
	// the commands the references read in its place have.
	if !value.negated && !isLoneBraceGroup(value) && !aliased {
		if r.errTrapTriggers(result) {
			r.runTrap(ctx, trapERR, result.status)
		}
		if r.errExitTriggers(result) {
			result.control = flowExit
		}
	}
	// A signal that arrived while the pipeline ran has its trap run now; see
	// signal_inbox.go.
	if result.control == flowNone {
		result = r.deliverSignals(ctx, result)
	}
	return result
}

// runLoneAlias runs a pipeline that is one aliased command as its alias's text, which sets
// $PIPESTATUS for its own pipelines; see alias_expand.go.
func (r Runtime) runLoneAlias(ctx context.Context, value pipeline, savedStatus int) (lineResult, bool) {
	if len(value.commands) != 1 || r.readsWithoutExecuting() {
		return lineResult{}, false
	}
	simple, ok := value.commands[0].(simpleCommand)
	if !ok {
		return lineResult{}, false
	}
	return r.runAlias(ctx, simple, savedStatus)
}

func isLoneBraceGroup(value pipeline) bool {
	if len(value.commands) != 1 {
		return false
	}
	_, group := value.commands[0].(braceGroup)
	return group
}

func (r Runtime) errExitTriggers(result lineResult) bool {
	return r.options.errExit && !r.errExitSuppressed && result.control == flowNone && result.status != 0
}

// errTrapTriggers is where `trap ... ERR` runs: after a pipeline fails where `set -e`
// would act -- not in a condition, not on a term of `&&` or `||` before the last, not
// under `!` -- whether or not `set -e` is on. Inside a function the trap is not inherited
// unless `set -E`; the call's own failure fires it in the caller instead. Subshells and
// substitutions lose it the same way, in snapshot. Every case was measured, and busybox
// and bash agree on all of them.
func (r Runtime) errTrapTriggers(result lineResult) bool {
	if r.traps[trapERR] == "" || r.errExitSuppressed || result.control != flowNone || result.status == 0 {
		return false
	}
	return r.functionDepth == 0 || r.options.errTrace
}

// suppressingErrExit marks a nested execution as one of the contexts `set -e`
// does not act on. The flag is on the Runtime value, so it applies to
// everything the returned Runtime runs and to nothing else.
func (r Runtime) suppressingErrExit() Runtime {
	r.errExitSuppressed = true
	return r
}

func (r Runtime) executeTypedPipelineStages(ctx context.Context, value pipeline, savedStatus int) lineResult {
	if len(value.commands) == 1 {
		// A one-element $PIPESTATUS, as bash gives. Set here as well as in
		// runTokenPipeline because a single command reaches execution by two
		// routes -- the AST one and the token one -- and a read after a plain
		// command must not find the previous pipeline's leftovers by either.
		result := r.executeCommandNode(ctx, value.commands[0], savedStatus)
		r.recordPipeStatus(result.status)
		return result
	}
	if result, ended := r.enterStages(ctx, value, savedStatus); ended {
		return result
	}
	stages := make([]pipelineStageRun, len(value.commands))
	for index, command := range value.commands {
		command := command
		stages[index] = func(ctx context.Context, stage Runtime, status int) lineResult {
			// A simple command's DEBUG trap has run, in the shell; see enterStages.
			if simple, ok := command.(simpleCommand); ok && !stage.readsWithoutExecuting() {
				if result, aliased := stage.runAlias(ctx, simple, status); aliased {
					return result
				}
				stage.enterSimpleCommand(simple)
				return stage.runSimpleCommand(ctx, simple, status)
			}
			return stage.executeCommandNode(ctx, command, status)
		}
	}
	prepared, err := r.preparePipeline(ctx, stages)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	return r.executeTokenPipeline(ctx, prepared, savedStatus)
}
