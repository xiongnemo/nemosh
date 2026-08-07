package runtime

import "fmt"

// expansionState carries a fatal expansion failure back out of expandWord.
// expandWord returns fields rather than an error and has too many callers to
// give it one cheaply, so the failure is recorded here and read at the few
// places that expand on behalf of a command: its words, its redirect operands,
// a case subject, and a for-loop's word list.
//
// One per snapshot, so a pipeline stage, a subshell, or a background worker
// cannot see another's.
type expansionState struct {
	unsetParameter bool
}

// reportUnsetParameter is the `set -u` path. POSIX says expanding an unset
// parameter writes a message and exits a non-interactive shell. busybox ash
// words it `NAME: parameter not set` (varunset, shell/ash.c:8269) and leaves
// status 2 behind (ash_msg_and_raise_error, shell/ash.c:1803).
//
// Only the first one is reported: a single command can expand several unset
// names, and the shell is leaving after the first of them anyway.
func (r Runtime) reportUnsetParameter(name string) {
	if !r.options.noUnset || r.expansion.unsetParameter {
		return
	}
	r.expansion.unsetParameter = true
	fmt.Fprintf(r.streams.Stderr, "nemosh: %s: parameter not set\n", name)
}

// expansionFailed reports whether the expansions since the last command hit
// something fatal, and clears the record so the next command starts clean.
func (r Runtime) expansionFailed() bool {
	if !r.expansion.unsetParameter {
		return false
	}
	r.expansion.unsetParameter = false
	return true
}

// unsetParameterResult is what every checkpoint returns: status 2 from
// busybox's error path, and an exit rather than a status because POSIX makes an
// unset parameter fatal to a non-interactive shell.
func unsetParameterResult() lineResult {
	return lineResult{status: 2, control: flowExit}
}
