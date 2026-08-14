package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

func (r Runtime) executeTypedScript(ctx context.Context, script Script) (int, flowControl) {
	return r.executeProgram(ctx, script.program, 0)
}

// executeTypedScriptFrom runs a script whose `$?` starts at a status decided by
// the caller rather than at zero.
//
// A command substitution needs this. Its script is a separate execution, so it
// used to begin at zero however the shell was doing, and `$(echo $?)` answered
// zero even when the previous command had failed. That is how a prompt like
// `$(prompt_info $?)` -- which every startup file with a failure indicator uses
// -- silently lost the exit code it exists to show.
func (r Runtime) executeTypedScriptFrom(ctx context.Context, script Script, savedStatus int) (int, flowControl) {
	return r.executeProgram(ctx, script.program, savedStatus)
}

func (r Runtime) executeProgram(ctx context.Context, program []programNode, savedStatus int) (int, flowControl) {
	status := savedStatus
	for _, item := range program {
		result := r.executeNode(ctx, item, status)
		status = result.status
		if result.control != flowNone {
			return status, result.control
		}
		if r.lifecycle.exitSuppressed {
			return status, flowExec
		}
		if ctx.Err() != nil {
			return contextStatus(ctx), flowNone
		}
	}
	return status, flowNone
}

func (r Runtime) executeNode(ctx context.Context, node programNode, savedStatus int) lineResult {
	switch value := node.(type) {
	case backgroundNode:
		return r.launchBackground(func(worker Runtime) lineResult {
			return worker.executeNode(worker.jobScope.ctx, value.value, savedStatus)
		})
	case listNode:
		return r.executeTypedList(ctx, value.value, savedStatus)
	case functionDefinition:
		r.functions[value.name] = value
		return lineResult{status: 0}
	case ifNode:
		return r.executeTypedIf(ctx, value, savedStatus)
	case loopNode:
		return r.executeTypedLoop(ctx, value, savedStatus)
	case caseNode:
		return r.executeTypedCase(ctx, value, savedStatus)
	default:
		return lineResult{status: 2}
	}
}

func (r Runtime) executeTypedList(ctx context.Context, item list, savedStatus int) lineResult {
	status := savedStatus
	for _, entry := range item.items {
		var result lineResult
		if entry.background {
			value := entry.value
			saved := status
			result = r.launchBackground(func(worker Runtime) lineResult {
				return worker.executeTypedAndOr(worker.jobScope.ctx, value, saved)
			})
		} else {
			result = r.executeTypedAndOr(ctx, entry.value, status)
		}
		status = result.status
		if result.control != flowNone {
			return result
		}
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
	}
	return lineResult{status: status}
}

func (r Runtime) launchBackground(run func(Runtime) lineResult) lineResult {
	worker, err := r.snapshot(r.jobScope.ctx)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	return r.launchBackgroundSnapshot(worker, run)
}

func (r Runtime) launchBackgroundSnapshot(worker Runtime, run func(Runtime) lineResult) lineResult {
	worker.traps = map[trapName]string{}
	if err := worker.fds.bindBorrowedReader(0, bytes.NewReader(nil)); err != nil {
		worker.jobScope.cancelAndDrain()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, worker.fds.closeAll()))
		return lineResult{status: 1}
	}
	// The worker's own scope cancel is what `kill %N` will reach for.
	record, err := r.jobScope.registerCancellable(worker.jobScope.cancel)
	if err != nil {
		worker.jobScope.cancelAndDrain()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, worker.fds.closeAll()))
		return lineResult{status: 1}
	}
	go func() {
		result := run(worker)
		worker.jobScope.cancelAndDrain()
		if err := worker.fds.closeAll(); err != nil && result.status == 0 {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
			result.status = 1
		}
		r.jobScope.complete(record, result.status)
	}()
	return lineResult{status: 0}
}

func (r Runtime) executeTypedAndOr(ctx context.Context, item andOr, savedStatus int) lineResult {
	status := savedStatus
	for index, pipeline := range item.pipelines {
		if index > 0 {
			operator := item.operators[index-1]
			if operator == tokenAndIf && status != 0 || operator == tokenOrIf && status == 0 {
				continue
			}
		}
		// Only the last command of an and-or list answers to `set -e`; the
		// earlier ones are what the operators are there to test.
		stage := r
		if index < len(item.pipelines)-1 {
			stage = r.suppressingErrExit()
		}
		result := stage.executeTypedPipeline(ctx, pipeline, status)
		status = result.status
		if result.control != flowNone {
			return result
		}
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
	}
	return lineResult{status: status}
}
