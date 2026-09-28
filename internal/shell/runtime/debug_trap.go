package runtime

import "context"

// The DEBUG trap is bash's; busybox-w32 has none. It runs before each simple command, before
// each turn of a for or select loop, before each of an arithmetic for's three parts, and before
// a case -- not before an if, a while or a group, whose commands have their own -- with $LINENO
// the command's line and $BASH_COMMAND the command as written. $? is what it was before the trap.
// It does not run inside itself, but does inside the other traps. A failure in it is only a
// failure: `set -e` makes it end the shell, even in a condition, where the command it ran for
// would not have; so does an `exit` in it. It was refused as not implemented, for want of
// $BASH_COMMAND.
//
// A function, a sourced file and a subshell do not inherit it unless `set -T`: from the first
// two it is hidden, as bash hides it, so `trap -p` there shows none, and one they set stays set
// after them; from a subshell it is dropped as ERR is (inheritedTraps). A pipeline forks its
// stages, and bash runs a simple command's trap before the fork: so here each stage that is a
// simple command has its trap run in the shell, in order, before the pipeline starts, and its
// output is the shell's, not the pipe's. A job that is one pipeline has the same, and an and-or
// list or a group sent to the background has none. Every case was measured in bash 5.3.

// debugTrap runs the trap for the command the shell is about to run, and answers whether that
// ended the shell, with how.
func (r Runtime) debugTrap(ctx context.Context, savedStatus int) (lineResult, bool) {
	if r.traps[trapDEBUG] == "" || r.trapRunning[trapDEBUG] {
		return lineResult{}, false
	}
	defer r.enterLine(r.currentLine())
	r.errExitSuppressed = false
	result := r.runTrap(ctx, trapDEBUG, savedStatus)
	switch result.control {
	case flowExit, flowAbort, flowExec:
		return result, true
	}
	return lineResult{}, false
}

// debugTrapHead is the trap for a compound command's head, which is $BASH_COMMAND while it
// runs, printed only if there is a trap to run.
func (r Runtime) debugTrapHead(ctx context.Context, line int, head func() string, savedStatus int) (lineResult, bool) {
	if r.traps[trapDEBUG] == "" || r.trapRunning[trapDEBUG] {
		return lineResult{}, false
	}
	r.enterLine(line)
	r.enterHead(head())
	return r.debugTrap(ctx, savedStatus)
}

// enterStages makes each stage of a pipeline that is a simple command the one running in turn,
// in the shell, before any starts, as bash does before it forks each, and runs the DEBUG trap
// for it. The stages then run without it; see executeTypedPipelineStages. So an ERR trap after
// the pipeline has the last such stage's $LINENO and $BASH_COMMAND, as bash's has: both were the
// command's before the pipeline, since the stages ran in snapshots of their own. busybox-w32's
// $LINENO there is that line before too, which is a line left stale rather than a choice, so
// bash's answer is the one taken.
func (r Runtime) enterStages(ctx context.Context, value pipeline, savedStatus int) (lineResult, bool) {
	for _, command := range value.commands {
		if simple, ok := command.(simpleCommand); ok {
			r.enterSimpleCommand(simple)
			if result, ended := r.debugTrap(ctx, savedStatus); ended {
				return result, true
			}
		}
	}
	return lineResult{}, false
}

// debugTrapJob is the trap for a job's commands, which bash runs before it forks them when
// the job is one pipeline, and not at all when it is an and-or list, which it forks whole.
func (r Runtime) debugTrapJob(ctx context.Context, value andOr, savedStatus int) (lineResult, bool) {
	if len(value.pipelines) != 1 || r.traps[trapDEBUG] == "" {
		return lineResult{}, false
	}
	return r.enterStages(ctx, value.pipelines[0], savedStatus)
}

// hideDebugTrap takes the trap away from a function's body or a sourced file, which do not
// inherit it, and answers with what to put back.
func (r Runtime) hideDebugTrap() (string, bool) {
	if r.options.funcTrace {
		return "", false
	}
	action, set := r.traps[trapDEBUG]
	delete(r.traps, trapDEBUG)
	return action, set
}

// restoreDebugTrap puts back the trap hideDebugTrap took, unless the body set one of its own.
func (r Runtime) restoreDebugTrap(hidden string, wasSet bool) {
	if _, set := r.traps[trapDEBUG]; wasSet && !set {
		r.traps[trapDEBUG] = hidden
	}
}

// runningInSnapshot is the trap a snapshot is inside of: the DEBUG trap, when one is taken while
// it runs, as bash's subshell forked there is still in it. Without that, a trap `set -T` passes on
// that runs `$(...)` ran again inside it, for ever.
func (r Runtime) runningInSnapshot() map[trapName]bool {
	if r.trapRunning[trapDEBUG] {
		return map[trapName]bool{trapDEBUG: true}
	}
	return map[trapName]bool{}
}

// loopHead is a for or select loop's head as $BASH_COMMAND gives it, one over the arguments
// with its `in "$@"`.
func loopHead(node loopNode) func() string {
	return func() string {
		var printer scriptPrinter
		if node.overArguments {
			return printer.loopHeader(node) + ` in "$@"`
		}
		return printer.loopHeader(node)
	}
}

// arithmeticHead is a part of an arithmetic for as $BASH_COMMAND gives it: `((i < 3))`, and
// `((1))` for an empty part, which bash reads as 1.
func arithmeticHead(part string) func() string {
	return func() string {
		if part == "" {
			part = "1"
		}
		return "((" + part + "))"
	}
}

// caseHead is a case's head as $BASH_COMMAND gives it, blank and all.
func caseHead(node caseNode) func() string {
	return func() string { return "case " + printWord(node.word) + " in " }
}
