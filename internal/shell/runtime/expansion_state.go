package runtime

import (
	"errors"
	"fmt"
	"slices"
)

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
	// shellErrorStatus is the status the raised error ends the script with when it is not
	// busybox's usual 2: a special builtin's failed redirection is 1. See raiseShellErrorWith.
	shellErrorStatus int
	// discard is set with shellError by a failed glob under failglob, which abandons the
	// command rather than the shell; see failglob.go.
	discard bool
	// skipLine is set with discard by an error `set -e` does not end the script for; see
	// flowSkipLine.
	skipLine bool
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
	// command is the simple command running, which is $BASH_COMMAND, or head the compound
	// command's head the DEBUG trap runs for, and ran whether one has; see enterCommand.
	command simpleCommand
	head    string
	ran     bool
	// processSubstitutions are the temporary files `<(cmd)` created for the command being
	// expanded. Held until the command has run, because the consumer opens them in
	// between; see process_substitution.go.
	processSubstitutions []string
	// outputPipes are the pipes `>(cmd)` handed the command being expanded, to let go of
	// once it has run; see output_substitution.go.
	outputPipes []*substitutionPipe
}

func (state *expansionState) registerProcessSubstitution(path string) {
	state.processSubstitutions = append(state.processSubstitutions, path)
}

// heldSubstitutions is where the next `<(cmd)` file and `>(cmd)` pipe will be registered. A
// construct that expands words notes it as it begins, and lets go of what came after it as it
// ends: a simple command, `[[ ]]`, a compound command's redirections, and the words of `for`,
// `select` and `case`. Each let go of everything registered so far, so the first command to
// end inside `while ... done < <(cmd)` took the loop's own input, which Windows would not
// remove while it was open -- 306 were in %TEMP% by 2026-11-19 -- and the first turn of
// `for f in <(a) <(b)` took both.
type heldSubstitutions struct{ files, pipes int }

func (state *expansionState) held() heldSubstitutions {
	return heldSubstitutions{files: len(state.processSubstitutions), pipes: len(state.outputPipes)}
}

// takeSubstitutionsFrom hands over what was registered from mark on, and forgets it.
func (state *expansionState) takeSubstitutionsFrom(mark heldSubstitutions) ([]string, []*substitutionPipe) {
	return takeRegisteredFrom(&state.processSubstitutions, mark.files), takeRegisteredFrom(&state.outputPipes, mark.pipes)
}

func takeRegisteredFrom[T any](list *[]T, mark int) []T {
	if mark >= len(*list) {
		return nil
	}
	taken := slices.Clone((*list)[mark:])
	clear((*list)[mark:])
	*list = (*list)[:mark]
	return taken
}

func newExpansionState() *expansionState {
	return &expansionState{warnedDebugChannels: map[string]bool{}}
}

// snapshotExpansionState is a snapshot's own, on the line its shell is running: what a
// subshell or a substitution says before a command of its own has run, `$(< missing)`'s
// failure, names that line. It named line 1.
func (r Runtime) snapshotExpansionState() *expansionState {
	state := newExpansionState()
	if r.expansion != nil {
		state.line = r.expansion.line
	}
	return state
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
	// A read-only name an expansion assigned has been said, and in bash mode raised nothing:
	// the line is abandoned, status 1, as bash's report_error abandons it.
	if errors.Is(err, errReadonlyTarget) {
		r.expansion.shellError, r.expansion.discard = true, true
		return
	}
	if r.BashMode() {
		r.reportExpansionErrorAsBash(err)
		return
	}
	r.expansion.shellError = true
	fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
}

// skipLineFor says an expansion error bash recovers from, and abandons the rest of the line
// with status 1, `set -e` or not; see flowSkipLine.
func (r Runtime) skipLineFor(err error) {
	if r.expansion.shellError {
		return
	}
	r.expansion.shellError, r.expansion.discard, r.expansion.skipLine = true, true, true
	fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
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
	if r.BashMode() {
		// bash's words and status: the script ends with 1, a command string with 127.
		r.expansion.shellErrorStatus = r.bashFatalStatus()
		fmt.Fprintf(r.streams.Stderr, "%s%s: unbound variable\n", r.diagnosticPrefix(), name)
		return
	}
	fmt.Fprintf(r.streams.Stderr, "%s%s: parameter not set\n", r.diagnosticPrefix(), name)
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

// raiseShellErrorWith is raiseShellError for an error that ends the script with a status of
// its own.
func (r Runtime) raiseShellErrorWith(status int) {
	r.expansion.shellError = true
	r.expansion.shellErrorStatus = status
}

// shellErrorResult is what every checkpoint returns: status 2 from busybox's
// error path, and an abort rather than a status because POSIX makes a shell
// error fatal to a non-interactive shell -- and only to the line, at a prompt.
// See flowAbort. A failed glob under failglob is status 1 and flowDiscard instead.
func (r Runtime) shellErrorResult() lineResult {
	if r.expansion.discard {
		control := flowDiscard
		if r.expansion.skipLine {
			control = flowSkipLine
		}
		r.expansion.discard, r.expansion.skipLine = false, false
		status := 1
		if r.expansion.shellErrorStatus != 0 {
			// bash mode's shift with too many operands, which abandons the line with 2.
			status, r.expansion.shellErrorStatus = r.expansion.shellErrorStatus, 0
		}
		return lineResult{status: status, control: control}
	}
	status := 2
	if r.expansion.shellErrorStatus != 0 {
		status, r.expansion.shellErrorStatus = r.expansion.shellErrorStatus, 0
	}
	return lineResult{status: status, control: flowAbort}
}

// abortOnShellError is the checkpoint after a simple command has run. A builtin
// answers only a status, so one that raised a shell error -- `export R=2` on a
// readonly R, or the `R=2` in front of any command -- is turned into the abort here.
func (r Runtime) abortOnShellError(result lineResult) lineResult {
	if r.shellErrorRaised() {
		return r.shellErrorResult()
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
