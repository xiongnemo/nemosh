package runtime

import "context"

// The RETURN trap is bash's: it runs as a function returns, or as a sourced file
// finishes. busybox has none, and here it was an invalid signal specification, so
// `f() { trap cleanup RETURN; ...; }` -- a function tidying up after itself however it
// leaves -- never tidied.
//
// A function does not inherit the trap unless `set -T` says it should. So the trap that
// fires as a call returns is the one set during it, and it stays set afterwards, as in
// bash: `trap -p RETURN` after the call still shows it, and the next function called
// does not fire it, because that one hides it in turn.

// hideReturnTrap takes the trap away from a call that does not inherit it, and answers
// with what to put back.
func (r Runtime) hideReturnTrap() (string, bool) {
	if r.options.funcTrace {
		return "", false
	}
	action, set := r.traps[trapRETURN]
	delete(r.traps, trapRETURN)
	return action, set
}

// finishReturnTrap runs the trap the call set, with the call's status as $?, or puts back
// the one it hid. Not after `exit` or an aborted script, where bash does not run it.
func (r Runtime) finishReturnTrap(ctx context.Context, hidden string, wasSet bool, result lineResult) {
	if _, set := r.traps[trapRETURN]; set {
		if result.control == flowNone || result.control == flowReturn {
			r.runTrap(ctx, trapRETURN, result.status)
		}
		return
	}
	if wasSet {
		r.traps[trapRETURN] = hidden
	}
}
