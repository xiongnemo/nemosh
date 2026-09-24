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
		body := jobBody(value.value)
		if r.processJobsEnabled() {
			return r.launchProcessJob(body)
		}
		return r.launchBackground(func(worker Runtime) lineResult {
			return worker.executeNode(worker.jobScope.ctx, body, savedStatus)
		})
	case listNode:
		return r.executeTypedList(ctx, value.value, savedStatus)
	case functionDefinition:
		value.file = r.currentFile()
		r.functions[value.name] = value
		return lineResult{status: 0}
	case ifNode:
		return r.executeTypedIf(ctx, value, savedStatus)
	case loopNode:
		return r.executeTypedLoop(ctx, value, savedStatus)
	case caseNode:
		return r.executeTypedCase(ctx, value, savedStatus)
	case coprocNode:
		return r.executeCoproc(ctx, value, savedStatus)
	default:
		return lineResult{status: 2}
	}
}

func (r Runtime) executeTypedList(ctx context.Context, item list, savedStatus int) lineResult {
	status := savedStatus
	for _, entry := range item.items {
		var result lineResult
		if entry.background && r.processJobsEnabled() {
			result = r.launchProcessJob(listNode{value: list{items: []listItem{{value: jobAndOr(entry.value)}}}})
		} else if entry.background {
			value := jobAndOr(entry.value)
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

// jobBody is what a job runs. A subshell that is the whole of the job runs as the job
// itself, as bash runs `( list ) &` in the one process it forks: a trap the list sets is
// then the job's, and `kill $!` reaches it. The job is as isolated as the subshell was.
func jobBody(node programNode) programNode {
	value, ok := node.(listNode)
	if !ok || len(value.value.items) != 1 || value.value.items[0].background {
		return node
	}
	return listNode{value: list{items: []listItem{{value: jobAndOr(value.value.items[0].value)}}}}
}

func jobAndOr(item andOr) andOr {
	if len(item.pipelines) != 1 || item.pipelines[0].negated || len(item.pipelines[0].commands) != 1 {
		return item
	}
	subshell, ok := item.pipelines[0].commands[0].(subshellCommand)
	if !ok {
		return item
	}
	group := braceGroup{body: subshell.body, redirects: subshell.redirects}
	return andOr{pipelines: []pipeline{{commands: []commandNode{group}}}}
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
	// A job's table is its own, as a subshell's is.
	worker.jobScope.outer = nil
	// A job is addressed by `kill` on its own, so it has an inbox of its own.
	worker.signals = newSignalInbox()
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
	record.deliver = worker.signals.offer
	// `$!` for a job that is a goroutine -- NEMOSH_JOBS=goroutine, or a runtime with a
	// registry of its own -- is a **job specification** and not a process id: there is
	// no pid to report (a job that is a process reports its own, job_process.go). Naming
	// the job keeps the two things `$!` is actually used for working -- `kill $!` and
	// `wait $!` both take `%N` -- where a number would have been a pid-shaped lie that
	// `kill` would apply to some other process entirely.
	r.vars["!"] = fmt.Sprintf("%%%d", record.id)
	r.markVarMutation("!")
	// Said out loud at a prompt, the way busybox says `[1] 19676`. The number is real and
	// the pid is not -- for the reason just above -- so the line carries the handle that
	// works and what to do with it, rather than a pid-shaped lie. On stderr, as busybox
	// puts it: on stdout it would end up inside `x=$(cmd &)`. A script gets nothing,
	// because it wants its output rather than a commentary.
	if r.interactive.session {
		fmt.Fprintf(r.streams.Stderr, "[%d] started; kill %%%d to stop it\n", record.id, record.id)
	}
	go func() {
		// Guarded here rather than relying on a defer further down: complete() is
		// not deferred, so a panic in run left the parent's wait with nobody to
		// answer it -- a hang, not a crash.
		result := worker.guardedRun("running a background job", func() lineResult {
			result := run(worker)
			// A job starts with no traps, so an EXIT trap it has is one it set, and it
			// runs as the job ends, as a subshell's does -- in both references. It never
			// ran; the process launcher found that by running it.
			if result.control != flowExec {
				worker.runOwnExitTrap(worker.jobScope.ctx, "", result.status)
			}
			return result
		})
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
