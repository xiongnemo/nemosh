package runtime

import (
	"context"
	"fmt"
)

type InteractiveResult struct {
	Status int
	Exited bool
}

type interactiveState struct {
	status int
	closed bool
}

func (r *Runtime) RunInteractive(ctx context.Context, script Script) InteractiveResult {
	if r.initErr != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", r.initErr)
		r.interactive.status = 1
		return InteractiveResult{Status: 1, Exited: r.interactive.closed}
	}
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
