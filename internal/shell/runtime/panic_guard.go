package runtime

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// A panic in this shell is a defect in this shell, and until now it reached the
// person using it as a Go goroutine dump. That is the wrong output twice over: it
// says nothing they can act on, and in an interactive session it took the whole
// shell down along with every variable and every directory they had set up.
//
// It is not hypothetical. Leaving the array store out of the snapshot constructor
// made `(a=(1 2))` -- eight characters -- print a stack trace and exit, and the
// pipeline spelling of it panicked on a *background* goroutine, where a recover in
// main could never have caught it.
//
// So there are four guards, and the placement is the whole design:
//
//   - RunScript, which covers everything synchronous. In an interactive session
//     that is per command, so the session survives and the next prompt appears.
//   - each pipeline stage goroutine, which must also record a failing status: its
//     `wait.Done()` was already deferred, so the shell would not have hung, but
//     the stage's result would have stayed the zero value and reported success.
//   - each background job goroutine, which must still call jobScope.complete.
//     That call is not deferred, so a panic there left `wait` waiting forever --
//     a hang, which is worse than the crash it replaced.
//   - main, as the last resort for anything before or after the runtime exists.
//
// The trace is not printed by default, for the reason diagnostic.go gives about
// detail: it carries host paths, and the behavior corpus compares output byte for
// byte. The hint says how to ask for it.

// internalErrorStatus is what a panicking command reports.
//
// 70 is EX_SOFTWARE from sysexits.h, "an internal software error has been
// detected". Nonzero matters more than the particular number -- it lets `set -e`
// and `||` react to a defect instead of treating it as success -- but a
// distinctive one means a bug report can name it.
const internalErrorStatus = 70

// guardedRun runs work, turning a panic into a failing result and a diagnostic.
func (r Runtime) guardedRun(where string, work func() lineResult) (result lineResult) {
	defer func() {
		if value := recover(); value != nil {
			r.reportPanic(where, value, debug.Stack())
			result = lineResult{status: internalErrorStatus}
		}
	}()
	return work()
}

// guardedStatus is guardedRun for the callers that deal in a status alone.
func (r Runtime) guardedStatus(where string, work func() int) (status int) {
	defer func() {
		if value := recover(); value != nil {
			r.reportPanic(where, value, debug.Stack())
			status = internalErrorStatus
		}
	}()
	return work()
}

func (r Runtime) reportPanic(where string, value any, stack []byte) {
	details := []string(nil)
	if r.debugEnabled(debugPanic) {
		details = strings.Split(strings.TrimRight(string(stack), "\n"), "\n")
	}
	writeInternalError(r.streams.Stderr, where, value, details)
}

// writeInternalError is shared with main, which has no Runtime to ask about debug
// channels and reads the environment for itself.
func writeInternalError(stderr io.Writer, where string, value any, details []string) {
	fmt.Fprintf(stderr, "nemosh: internal error while %s: %v\n", where, value)
	if len(details) == 0 {
		fmt.Fprintln(stderr, "hint: this is a defect in nemosh, not in what you typed;"+
			" set NEMOSH_DEBUG=panic for the trace and please report it")
		return
	}
	for _, line := range details {
		fmt.Fprintf(stderr, "  %s\n", line)
	}
}

// ReportInternalError writes the same diagnostic from outside the package, for the
// guard in main. withTrace is whether the reader asked for the detail.
func ReportInternalError(stderr io.Writer, where string, value any, stack []byte, withTrace bool) {
	details := []string(nil)
	if withTrace {
		details = strings.Split(strings.TrimRight(string(stack), "\n"), "\n")
	}
	writeInternalError(stderr, where, value, details)
}

// InternalErrorStatus is what main should exit with, so the number is stated once.
func InternalErrorStatus() int { return internalErrorStatus }
