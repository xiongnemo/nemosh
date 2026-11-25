package applets

import "context"

// ProgramRunner finds a program for the applets that run a command -- env, xargs, find's -exec
// and -ok, and awk's system(), getline and pipes -- when no applet has the name. An applet never
// starts a process itself (see process_boundary_test.go): the shell does, through the Applet this
// hands back, with all it knows of launching one on Windows, the PATH search and its suffixes,
// ComSpec for a batch file, the quoting, a script's interpreter. So `xargs git add`, `find . -exec
// gofmt -l {} +` and `env GIT_PAGER=cat git log` run git and gofmt, where each was `not found`
// whatever PATH held, as busybox-w32's would have run them.
type ProgramRunner interface {
	// Program is the program a command of that name runs, if the shell can find one, looked up
	// on the PATH of the ProcessView in ctx, which env may have changed. Its Run takes the
	// environment from the ProcessView in the context it is given.
	Program(ctx context.Context, name string) (Applet, bool)
}

type programRunnerKey struct{}

// WithProgramRunner gives the applets ctx reaches a runner for the programs they run.
func WithProgramRunner(ctx context.Context, runner ProgramRunner) context.Context {
	return context.WithValue(ctx, programRunnerKey{}, runner)
}

// programFor is the program ctx's runner has for name, if it has a runner and the program.
func programFor(ctx context.Context, name string) (Applet, bool) {
	runner, ok := ctx.Value(programRunnerKey{}).(ProgramRunner)
	if !ok || runner == nil {
		return nil, false
	}
	return runner.Program(ctx, name)
}

type detachedLaunchKey struct{}

// WithDetachedLaunch asks the shell to launch the program run under ctx out of the session the
// shell is in, as nohup asks: out of every Job Object that allows it to leave, which is how an
// sshd session ends what it started, and off the console, whose closing ends what is on it.
func WithDetachedLaunch(ctx context.Context) context.Context {
	return context.WithValue(ctx, detachedLaunchKey{}, true)
}

// DetachedLaunch is whether ctx asks for the launch WithDetachedLaunch does.
func DetachedLaunch(ctx context.Context) bool {
	detached, _ := ctx.Value(detachedLaunchKey{}).(bool)
	return detached
}

// commandFor is what a command an applet runs names: an applet, as busybox-w32 prefers one, or
// else a program.
func commandFor(ctx context.Context, name string) (Applet, bool) {
	if applet, ok := DefaultRegistry.Lookup(name); ok {
		return applet, true
	}
	return programFor(ctx, name)
}
