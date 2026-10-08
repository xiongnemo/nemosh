package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The programs the applets that run a command reach for when no applet has the name: env's PROG,
// xargs's, find's -exec and -ok, awk's system() and pipes. applets.ProgramRunner says why the
// shell launches them and not the applet; here they are launched as runExternal launches a
// command -- the same lookup, the same script and batch-file handling, the same job of the
// working directory -- with the arguments, streams and environment the applet gives.

// Program is the program a command of that name would run, for an applet to run it, looked for
// on the PATH of the view in ctx: env's, when env set one, as env's exec searches the PATH it
// was given.
func (r Runtime) Program(ctx context.Context, name string) (applets.Applet, bool) {
	finder := r.programFinder(applets.ProcessViewFromContext(ctx))
	if _, err := finder.externalCommandPath(name); err != nil {
		return nil, false
	}
	return externalProgram{runtime: finder, name: name}, true
}

// programFinder is the shell as it looks a program up for view: on view's PATH, where the view is
// env's and has one.
func (r Runtime) programFinder(view applets.ProcessView) Runtime {
	if _, shells := view.(Runtime); !shells && view != nil {
		if path, set := view.LookupEnv("PATH"); set && path != "" {
			r.searchPath = path
		}
	}
	return r
}

type externalProgram struct {
	runtime Runtime
	name    string
}

func (p externalProgram) Name() string { return p.name }

// Run runs the program, its status the applet's error: none for 0, and the status otherwise, as
// an applet's failure carries one.
func (p externalProgram) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	status := p.runtime.runProgram(ctx, p.name, args, applets.ProcessViewFromContext(ctx), stdin, stdout, stderr)
	if status != 0 {
		return applets.ExitStatus(status)
	}
	return nil
}

// runProgram is runExternal for an applet's program. Its failures are said on the applet's
// stderr under the program's name, and its status is what runExternal's would be.
func (r Runtime) runProgram(ctx context.Context, name string, args []string, view applets.ProcessView,
	stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(status int, err error) int {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return status
	}
	directory, err := r.nativeWorkingDirectory()
	if err == nil {
		directory, err = requireAbsoluteNativePath("working directory", directory)
	}
	if err == nil {
		directory, err = launchWorkingDirectory(directory)
	}
	if err != nil {
		return fail(1, err)
	}
	executable, err := r.externalCommandPath(name)
	if err == nil {
		executable, err = requireAbsoluteNativePath("executable", executable)
	}
	var launchArgs []string
	if err == nil {
		executable, launchArgs, err = r.externalLaunchTarget(executable, args)
	}
	if errors.Is(err, errExternalNotFound) {
		return fail(127, err)
	}
	var cmd *exec.Cmd
	if err == nil {
		cmd, err = r.externalCommand(ctx, executable, launchArgs)
	}
	if err != nil {
		return fail(126, err)
	}
	cmd.Dir, cmd.Env = directory, r.programEnvironment(view)
	leased, release := externalStdin(ctx, stdin)
	defer release()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = leased, programStream(stdout), programStream(stderr)
	if err = r.startChild(cmd); err == nil {
		err = cmd.Wait()
	}
	r.recordChildCPU(cmd)
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		status, _ := processOutcome(exitErr.ProcessState)
		return status
	}
	if err != nil {
		if requiresElevation(err) {
			return fail(126, errors.New(elevationDiagnostic(name).message))
		}
		return fail(126, err)
	}
	return 0
}

// programEnvironment is what the program receives: the shell's environment, as a command's is,
// or the one env made of it, with the shell's path variables in the platform's spelling either
// way, so `env FOO=1 git` passes FOO on.
func (r Runtime) programEnvironment(view applets.ProcessView) []string {
	if _, shells := view.(Runtime); shells || view == nil {
		return r.childEnvironment()
	}
	return r.withNativePaths(view.Environ())
}

// programStream is the file a stream ends at, when it ends at one, so a program an applet runs
// writes to the console itself, as a command does, rather than through a pipe this process
// copies: git colours, and a pager knows the terminal. The interrupt wrapper is passed over; the
// program is ended by the context it runs under instead.
func programStream(writer io.Writer) io.Writer {
	if wrapped, ok := writer.(interruptibleWriter); ok {
		writer = wrapped.writer
	}
	if native := nativeWriter(writer); native != nil {
		return native
	}
	return writer
}
