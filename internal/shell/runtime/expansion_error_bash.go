package runtime

import (
	"errors"
	"fmt"
	"strings"
)

// An expansion that fails ends a busybox script, status 2, and a nemosh one under every name but
// bash. bash, outside POSIX mode, abandons the rest of the line, status 1, and goes on with the
// next -- an arithmetic error whatever `set -e` says, a bad substitution and `${1:=x}` unless it
// ends the script -- and `${x?}` ends the script with 1. Bash mode takes bash's; see
// docs/support-matrix.md, Bash mode.

// parameterAbsent is `${x?}`'s and `${x:?}`'s failure, the parameter unset or null: the one
// expansion error bash ends a script for.
type parameterAbsent string

func (e parameterAbsent) Error() string { return string(e) }

// reportExpansionErrorAsBash is reportExpansionError in bash mode: bash's words for a bad
// substitution, `${x!y}: bad substitution`, and the line abandoned, status 1, unless the error
// is `${x?}`'s, which ends the script with 1.
func (r Runtime) reportExpansionErrorAsBash(err error) {
	text := err.Error()
	if rest, ok := strings.CutPrefix(text, "bad substitution: "); ok && strings.HasPrefix(rest, "${") {
		text = rest + ": bad substitution"
	}
	fmt.Fprintf(r.streams.Stderr, "%s%s\n", r.diagnosticPrefix(), text)
	r.expansion.shellError = true
	var absent parameterAbsent
	if errors.As(err, &absent) {
		r.expansion.shellErrorStatus = r.bashFatalStatus()
		return
	}
	r.expansion.discard = true
}

// bashFatalStatus is what `${x?}`, or an unset name under set -u, ends a bash-mode shell with:
// 1, and 127 when it is a `-c` command string's own, outside any subshell, as bash 5.3 exits
// from its command string, and from a function it calls, where a script, standard input, a
// subshell and a `$(...)` end with 1.
func (r Runtime) bashFatalStatus() int {
	if strings.ContainsRune(r.options.invocation, 'c') && r.subshellDepth == 0 {
		return 127
	}
	return 1
}

// arithmeticExpansionFailed is the error of a `$((...))` being expanded: in bash mode the rest of
// the line is abandoned, status 1, `set -e` or not, as bash recovers from it; a read-only name it
// assigned, and every error under the other names, as reportExpansionError has it.
func (r Runtime) arithmeticExpansionFailed(err error) {
	if r.BashMode() && !errors.Is(err, errReadonlyTarget) {
		r.skipLineFor(err)
		return
	}
	r.reportExpansionError(err)
}
