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
	// shellError is a POSIX 2.8.1 shell error raised and not yet acted on -- an
	// unset parameter, an impossible expansion, an assignment to a readonly
	// variable. Raised wherever it happens and read at the checkpoints, which turn
	// it into flowAbort.
	shellError bool
	// warnedDebugChannels remembers which unknown NEMOSH_DEBUG names have
	// already been complained about, so the complaint does not bury the
	// diagnostics it is attached to.
	warnedDebugChannels map[string]bool
	// substitutionStatus is what the last command substitution of the command being expanded
	// exited with, and substitutions counts how many there were.
	//
	// Kept because POSIX makes an assignment-only command exit with it: `out=$(cmd)` completes
	// with cmd's status, which is what makes `out=$(cmd) || handler` work. Without this the
	// status was thrown away and every such assignment reported success -- so the commonest
	// error check in shell never fired.
	substitutionStatus int
	substitutions      int
	// line is the source line of the command running, which is $LINENO. Here because it is
	// per snapshot too: a background job counts its own lines while the shell goes on.
	line int
	// processSubstitutions are the temporary files `<(cmd)` created for the command being
	// expanded. Held until the command has run, because the consumer opens them in
	// between; see process_substitution.go.
	processSubstitutions []string
}

func (state *expansionState) registerProcessSubstitution(path string) {
	state.processSubstitutions = append(state.processSubstitutions, path)
}

// takeProcessSubstitutions hands over the paths and forgets them, so a second command does
// not try to remove the same files.
func (state *expansionState) takeProcessSubstitutions() []string {
	paths := state.processSubstitutions
	state.processSubstitutions = nil
	return paths
}

func newExpansionState() *expansionState {
	return &expansionState{warnedDebugChannels: map[string]bool{}}
}

// reportExpansionError is the path for a substitution that cannot be carried
// out at all -- an operator this shell does not implement, or a `${x:?message}`
// whose parameter is unset. POSIX makes both fatal to a non-interactive shell,
// and the alternative this replaces was worse than fatal: an unrecognised
// `${x%.txt}` used to expand to its own six characters and exit 0.
func (r Runtime) reportExpansionError(err error) {
	if r.expansion.shellError {
		return
	}
	r.expansion.shellError = true
	fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
}

// reportUnsetParameter is the `set -u` path. POSIX says expanding an unset
// parameter writes a message and exits a non-interactive shell. busybox ash
// words it `NAME: parameter not set` (varunset, shell/ash.c:8269) and leaves
// status 2 behind (ash_msg_and_raise_error, shell/ash.c:1803).
//
// Only the first one is reported: a single command can expand several unset
// names, and the shell is leaving after the first of them anyway.
func (r Runtime) reportUnsetParameter(name string) {
	if !r.options.noUnset || r.expansion.shellError {
		return
	}
	r.expansion.shellError = true
	fmt.Fprintf(r.streams.Stderr, "nemosh: %s: parameter not set\n", name)
}

// shellErrorRaised reports whether anything since the last checkpoint raised a
// shell error, and clears the record so the next command starts clean.
func (r Runtime) shellErrorRaised() bool {
	if !r.expansion.shellError {
		return false
	}
	r.expansion.shellError = false
	return true
}

// raiseShellError records a shell error whose diagnostic has already been written.
func (r Runtime) raiseShellError() {
	r.expansion.shellError = true
}

// shellErrorResult is what every checkpoint returns: status 2 from busybox's
// error path, and an abort rather than a status because POSIX makes a shell
// error fatal to a non-interactive shell -- and only to the line, at a prompt.
// See flowAbort.
func shellErrorResult() lineResult {
	return lineResult{status: 2, control: flowAbort}
}

// abortOnShellError is the checkpoint after a simple command has run. A builtin
// answers only a status, so one that raised a shell error -- `export R=2` on a
// readonly R, or the `R=2` in front of any command -- is turned into the abort here.
func (r Runtime) abortOnShellError(result lineResult) lineResult {
	if r.shellErrorRaised() {
		return shellErrorResult()
	}
	return result
}

// recordSubstitution notes what a command substitution exited with.
func (state *expansionState) recordSubstitution(status int) {
	state.substitutions++
	state.substitutionStatus = status
}

// substitutionMark is the substitution count before a command is expanded, so the caller can tell
// whether *this* command performed one rather than inheriting an older answer.
func (state *expansionState) substitutionMark() int { return state.substitutions }

// substitutionStatusSince reports the status of the last substitution performed since the mark, and
// whether there was one at all.
func (state *expansionState) substitutionStatusSince(mark int) (int, bool) {
	if state.substitutions == mark {
		return 0, false
	}
	return state.substitutionStatus, true
}
