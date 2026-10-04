package runtime

// lineResult is what running a command answers: its status, and the control transfer it
// asked for, if any, which unwinds as far as the construct that consumes it.
type lineResult struct {
	status  int
	control flowControl
}

type flowControl int

const (
	flowNone flowControl = iota
	flowExit
	flowBreak
	flowContinue
	flowExec
	flowReturn
	// flowAbort is a shell error in the sense of POSIX 2.8.1 -- an unset parameter
	// under `set -u`, an assignment to a readonly variable. It unwinds exactly as
	// far as flowExit does, and differs in one place: a person at a prompt. There
	// the shell "shall write a diagnostic message ... without exiting", so the rest
	// of the line is abandoned and the session goes on. These used to be flowExit,
	// and `set -u; echo $nope` typed at a prompt closed the terminal.
	flowAbort
	// flowDiscard is a pattern that matched nothing under `shopt -s failglob`. It unwinds
	// as flowAbort does, with status 1, and a script goes on with its next command, as a
	// session does, unless `set -e` ends it; see failglob.go.
	flowDiscard
	// flowSkipLine is flowDiscard for an error bash recovers from whatever `set -e` says,
	// one it reports with internal_error rather than report_error: an expansion's
	// `substring expression < 0`. The script goes on with its next line, status 1.
	flowSkipLine
)
