package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) executeTypedPipeline(ctx context.Context, value pipeline, savedStatus int) lineResult {
	result := r.executeTypedPipelineStages(ctx, value, savedStatus)
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
	if r.errTrapTriggers(result) && !value.negated {
		r.runTrap(ctx, trapERR, result.status)
	}
	if r.errExitTriggers(result) && !value.negated {
		result.control = flowExit
	}
	// A signal that arrived while the pipeline ran has its trap run now; see
	// signal_inbox.go.
	if result.control == flowNone {
		result = r.deliverSignals(ctx, result)
	}
	return result
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
	stages := make([]pipelineStageRun, len(value.commands))
	for index, command := range value.commands {
		command := command
		stages[index] = func(ctx context.Context, stage Runtime, status int) lineResult {
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
