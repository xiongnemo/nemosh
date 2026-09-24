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
	if parseErr == nil {
		status, control = r.executePrepared(ctx, prepared)
	}
	if parseErr != nil && control == flowNone {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", parseErr)
		if runExitTrap {
			r.runExitTrap(context.WithoutCancel(ctx), 2)
		}
		return 2, flowNone
	}
	if runExitTrap && control != flowExec {
		if status == 130 && isShellInterrupt(ctx) {
			r.runInterruptTrap(context.WithoutCancel(ctx), status)
		}
		r.runExitTrap(context.WithoutCancel(ctx), status)
	}
	if control == flowExec {
		r.lifecycle.exitSuppressed = true
	}
	return status, control
}

func (r Runtime) CloseBatch(savedStatus int) {
	if !r.lifecycle.exitSuppressed {
		r.runExitTrap(context.Background(), savedStatus)
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
}

func (r Runtime) executePrepared(ctx context.Context, script Script) (int, flowControl) {
	return r.executeTypedScript(ctx, script)
}

func (r Runtime) runExitTrap(ctx context.Context, savedStatus int) {
	r.runTrap(ctx, trapExit, savedStatus)
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
