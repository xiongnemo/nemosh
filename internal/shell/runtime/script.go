package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) RunScript(ctx context.Context, script string) int {
	if r.initErr != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", r.initErr)
		return 1
	}
	// The synchronous guard. In an interactive session this is per command, which
	// is what lets the session outlive a defect instead of dying with it.
	return r.guardedStatus("running a command", func() int {
		return r.runScript(ctx, script, true)
	})
}

func (r Runtime) runScript(ctx context.Context, script string, runExitTrap bool) int {
	status, _ := r.runScriptResult(ctx, script, 1, runExitTrap)
	return status
}

// runScriptResult parses script as beginning on source line first, for $LINENO: 1 for a
// script of its own, the running command's line for eval.
func (r Runtime) runScriptResult(ctx context.Context, script string, first int, runExitTrap bool) (int, flowControl) {
	prepared, parseErr := parseScriptAt(script, first)
	status := 0
	control := flowNone
	switch {
	// The script's own run, the one with an EXIT trap to run, is the top level; eval and
	// `.` are not.
	case parseErr == nil && runExitTrap:
		status, control = r.executeTopLevel(ctx, prepared.program)
	case parseErr == nil:
		status, control = r.executePrepared(ctx, prepared)
	}
	if parseErr != nil && control == flowNone {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", parseErr)
		status = 2
		if runExitTrap {
			status = r.runExitTrap(context.WithoutCancel(ctx), status)
		}
		return status, flowNone
	}
	if runExitTrap && control != flowExec {
		if status == 130 && isShellInterrupt(ctx) {
			r.runInterruptTrap(context.WithoutCancel(ctx), status)
		}
		// A signal that ended the script with a trap set for it: a final one (see
		// ReceiveSignals). One nothing caught has no trap to run.
		if signal, ok := ExitSignal(ctx); ok {
			r.runTrap(context.WithoutCancel(ctx), signalTraps[signal], status)
		}
		status = r.runExitTrap(context.WithoutCancel(ctx), status)
	}
	if control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	return status, control
}

// CloseBatch ends a shell that ran a script or a command, and answers the status it ends with,
// which an `exit` in its EXIT trap decides when there is one.
func (r Runtime) CloseBatch(savedStatus int) int {
	status := savedStatus
	if !r.lifecycle.exitSuppressed {
		status = r.runExitTrap(context.Background(), savedStatus)
	}
	r.jobScope.seal()
	// A descriptor an `exec` redirect opened belongs to the shell and outlives
	// every command that ran under it, so closing the shell is the only place
	// it can be released. Everything else in the table is borrowed and closing
	// it is a no-op. Reported before the table goes, since the report itself
	// needs a descriptor to come out of.
	if err := r.fds.closeAll(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
	}
	// After the descriptors, whose closing is the end of input a `>(cmd)` reading from an
	// `exec` redirect waits for: `exec > >(tee log)` has all of its log once this returns.
	if r.substitutions != nil {
		r.substitutions.Wait()
	}
	return status
}

func (r Runtime) executePrepared(ctx context.Context, script Script) (int, flowControl) {
	return r.executeTypedScript(ctx, script)
}

// runExitTrap runs the EXIT trap and answers the status the shell ends with: the one it had,
// or the one an `exit` in the trap gave, as both references end -- a script, a subshell and a
// command substitution alike. `trap 'exit 42' EXIT` ended with the status before the trap.
func (r Runtime) runExitTrap(ctx context.Context, savedStatus int) int {
	if result := r.runTrap(ctx, trapExit, savedStatus); result.control == flowExit {
		return result.status
	}
	return savedStatus
}

func (r Runtime) runInterruptTrap(ctx context.Context, savedStatus int) {
	r.runTrap(ctx, trapINT, savedStatus)
}

func (r Runtime) runTrap(ctx context.Context, name trapName, savedStatus int) lineResult {
	command := r.traps[name]
	if command == "" || r.trapRunning[name] {
		return lineResult{status: savedStatus}
	}
	if name == trapExit {
		delete(r.traps, name)
	}
	r.trapRunning[name] = true
	defer delete(r.trapRunning, name)
	prepared, err := r.parseHere(command)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "trap %s: %v\n", name, err)
		return lineResult{status: savedStatus}
	}
	status, control := r.executeProgram(ctx, prepared.program, savedStatus)
	return lineResult{status: status, control: control}
}
