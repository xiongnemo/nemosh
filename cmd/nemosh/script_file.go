package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// defaultScriptName is $0 when nothing else names the shell — a command string
// with no operands, or a script arriving on stdin.
const defaultScriptName = "nemosh"

// scriptInvocation is how a run names itself: $0 plus the positional parameters.
type scriptInvocation struct {
	name string
	args []string
}

// commandStringInvocation reads the operands after `-c command_string`. POSIX
// spells this `sh -c command_string [command_name [argument...]]`, so the first
// operand becomes $0 rather than $1.
func commandStringInvocation(operands []string) scriptInvocation {
	if len(operands) == 0 {
		return scriptInvocation{name: defaultScriptName}
	}
	return scriptInvocation{name: operands[0], args: operands[1:]}
}

// runScriptFile executes a script named on the command line. $0 is the operand
// exactly as the user wrote it, which is what a script echoing its own name in a
// usage message should print.
func (c command) runScriptFile(ctx context.Context, controller *interruptController, path string, args []string) error {
	rt := c.newRuntime()
	script, err := readScriptFile(rt, path)
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: can't open '%s': %v\n", path, openFailureReason(err))
		return exitStatus(127)
	}
	return c.runScriptWith(ctx, controller, rt, string(script), scriptInvocation{name: path, args: args})
}

// readScriptFile takes the operand through the shell's own path model, so every
// spelling the shell prints is one it accepts back: `pwd` answers
// /c/Users/nemo/work and `nemosh /c/Users/nemo/work/build.sh` has to run. It
// used to reach os.ReadFile unconverted, so that form failed with 127 while the
// shell went on printing it.
func readScriptFile(rt runtime.Runtime, path string) ([]byte, error) {
	resolved, err := rt.ResolveNemoshPath(path)
	if err != nil {
		return nil, err
	}
	if resolved.Device {
		return nil, errors.New("not a regular file")
	}
	return os.ReadFile(resolved.Native)
}

// openFailureReason drops the operation and path that fs.PathError repeats, so
// the diagnostic names the script once instead of three times.
func openFailureReason(err error) error {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return pathErr.Err
	}
	return err
}

func (c command) newRuntime() runtime.Runtime {
	return runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: c.stdin, Stdout: c.stdout, Stderr: c.stderr})
}

func (c command) runScriptAs(ctx context.Context, controller *interruptController, script string, invocation scriptInvocation) error {
	return c.runScriptWith(ctx, controller, c.newRuntime(), script, invocation)
}

// runScriptWith takes the runtime as a parameter because a script file has to
// resolve its own operand through one before there is anything to run.
func (c command) runScriptWith(ctx context.Context, controller *interruptController, rt runtime.Runtime, script string, invocation scriptInvocation) error {
	rt.SetArguments(invocation.name, invocation.args)
	executionCtx, clear := controller.context(ctx)
	status := rt.RunScript(executionCtx, script)
	clear()
	rt.CloseBatch(status)
	if status == 0 {
		return nil
	}
	return exitStatus(status)
}
