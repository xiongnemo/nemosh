package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type InteractiveResult struct {
	Status int
	Exited bool
}

type interactiveState struct {
	status int
	closed bool
	// session marks a shell that is talking to a person rather than running a script,
	// which is what decides whether backgrounding a command announces the job. Copied
	// along with the rest of this struct into every snapshot, which is what a background
	// launch deeper in an expansion needs -- and read-only after RunInteractive sets it,
	// so the copying that cost `$?` its status cannot cost this anything.
	session bool
	// linesRead is how many lines the session has parsed, which its $LINENO counts on from.
	linesRead int
}

// ParseSessionInput parses what a session has read so far, numbering it on from the lines
// before it: busybox's $LINENO counts a session's lines, and a function defined at a
// prompt reports the lines it was typed on. Input still waiting for more lines is not
// counted yet, because it comes back whole with them.
func (r *Runtime) ParseSessionInput(source string) (Script, error) {
	script, err := parseScriptAt(source, r.interactive.linesRead+1)
	if !errors.Is(err, ErrIncompleteScript) {
		r.interactive.linesRead += strings.Count(strings.TrimSuffix(source, "\n"), "\n") + 1
	}
	return script, err
}

// IgnoresEOF is `set -o ignoreeof`: an end of input at the prompt is refused rather than
// taken as `exit`.
func (r Runtime) IgnoresEOF() bool { return r.options.ignoreEOF }

func (r *Runtime) RunInteractive(ctx context.Context, script Script) InteractiveResult {
	if r.initErr != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", r.initErr)
		r.interactive.status = 1
		return InteractiveResult{Status: 1, Exited: r.interactive.closed}
	}
	r.interactive.session = true
	status := r.interactive.status
	control := flowNone
	if len(script.program) > 0 {
		status, control = r.executeProgram(ctx, script.program, status)
		if status == 130 && isShellInterrupt(ctx) {
			r.runInterruptTrap(context.WithoutCancel(ctx), status)
		}
		r.interactive.status = status
	}
	if control == flowExit {
		r.interactive.closed = true
		r.runExitTrap(context.WithoutCancel(ctx), status)
		r.jobScope.seal()
	}
	if control == flowExec {
		r.interactive.closed = true
		r.lifecycle.exitSuppressed = true
		r.jobScope.seal()
	}
	return InteractiveResult{Status: status, Exited: r.interactive.closed}
}

func (r *Runtime) ReportInteractiveParseError(err error) {
	fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
	r.interactive.status = 2
}

func (r *Runtime) CloseInteractive(ctx context.Context) int {
	status := r.interactive.status
	if !r.interactive.closed {
		r.interactive.closed = true
		if !r.lifecycle.exitSuppressed {
			r.runExitTrap(context.WithoutCancel(ctx), status)
		}
		r.jobScope.seal()
	}
	return status
}
