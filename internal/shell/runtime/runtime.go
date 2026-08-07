package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Runtime struct {
	initErr     error
	registry    applets.Registry
	functions   map[functionName]functionDefinition
	streams     Streams
	fds         *fdTable
	vars        map[string]string
	traps       map[trapName]string
	trapRunning map[trapName]bool
	params      *parameters
	options     *shellOptions
	expansion   *expansionState
	aliases     map[string]string
	// childCPU is shared by pointer across snapshots: a pipeline stage's
	// children are the shell's children too, and `times` in the parent has to
	// see what they used.
	childCPU *childCPUTime
	// locals belongs to the function call in progress and is nil outside one,
	// which is how `local` knows there is nothing to restore to.
	locals *localScope
	// errExitSuppressed marks the places POSIX 2.9.1 exempts from `set -e`: a
	// condition, a negated pipeline, and every command but the last of an
	// and-or list. It rides on the Runtime value rather than the shared options
	// pointer so entering one of those places cannot leak out of it.
	errExitSuppressed bool
	readonly          map[string]struct{}
	mutatedVars       map[string]struct{}
	mask              *fileModeMask
	sourceDepth       int
	functionDepth     int
	interactive       interactiveState
	paths             *pathState
	env               Environment
	jobScope          *jobScope
	lifecycle         *shellLifecycle
}

type shellLifecycle struct {
	exitSuppressed bool
}

type trapName string

const (
	trapExit trapName = "EXIT"
	trapINT  trapName = "INT"
)

func New(registry applets.Registry, streams Streams) Runtime {
	return NewWithState(registry, streams, hostState())
}

func NewRuntime(registry applets.Registry, streams Streams) (Runtime, error) {
	return NewRuntimeWithState(registry, streams, hostState())
}

func fillStreams(streams Streams) Streams {
	if streams.Stdin == nil {
		streams.Stdin = bytes.NewReader(nil)
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	mutex := &sync.Mutex{}
	streams.Stdout = synchronizedWriter{mutex: mutex, writer: streams.Stdout}
	streams.Stderr = synchronizedWriter{mutex: mutex, writer: streams.Stderr}
	return streams
}

type synchronizedWriter struct {
	mutex  *sync.Mutex
	writer io.Writer
}

func (w synchronizedWriter) Write(buffer []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.writer.Write(buffer)
}

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
)

func (r Runtime) runCommandWithRedirects(ctx context.Context, args []string) int {
	commandArgs, streams, cleanup, err := r.applyRedirects(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	commandRuntime, err := r.snapshotShared()
	if err != nil {
		cleanupErr := cleanup()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, cleanupErr))
		return 1
	}
	commandRuntime, err = commandRuntime.withStreams(streams)
	if err != nil {
		cleanupErr := cleanup()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, cleanupErr))
		return 1
	}
	status := commandRuntime.runCommand(ctx, commandArgs)
	commandRuntime.jobScope.drain()
	closeErr := commandRuntime.fds.closeAll()
	if err := cleanup(); err != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	if closeErr != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", closeErr)
		return 1
	}
	return status
}

func (r Runtime) runCommand(ctx context.Context, args []string) int {
	status := r.runCommandResolved(ctx, args, true)
	if ctx.Err() != nil {
		return contextStatus(ctx)
	}
	return status
}

func (r Runtime) runCommandResolved(ctx context.Context, args []string, allowFunctions bool) int {
	if allowFunctions && !isSpecialBuiltin(args[0]) {
		if name, ok := newFunctionName(args[0]); ok {
			if definition, found := r.functions[name]; found {
				return r.callFunction(ctx, definition, args[1:])
			}
		}
	}
	switch args[0] {
	case "alias":
		return r.alias(args[1:])
	case "unalias":
		return r.unalias(args[1:])
	case "local":
		return r.local(args[1:])
	case "type":
		return r.typeBuiltin(args[1:])
	case "let":
		return r.let(args[1:])
	case "times":
		return r.times()
	case ":":
		// The null command of POSIX 2.14: its arguments are expanded, which has
		// already happened by the time it gets here, and it returns zero.
		// `while :; do ... done` is the ordinary way to write an endless loop,
		// and without this it was a failed lookup for a program named `:`.
		return 0
	case ".", "source":
		return r.dot(ctx, args[1:])
	case "cd":
		return r.cd(args[1:])
	case "command":
		return r.command(ctx, args[1:])
	case "eval":
		return r.eval(ctx, args[1:])
	case "getopts":
		return r.getopts(args[1:])
	case "jobs":
		return r.jobs(args[1:])
	case "export":
		return r.export(args[1:])
	case "unset":
		return r.unset(args[1:])
	case "pwd":
		return r.pwd()
	case "read":
		return r.read(ctx, args[1:])
	case "readonly":
		return r.readonlyBuiltin(args[1:])
	case "set":
		return r.set(args[1:])
	case "shift":
		return r.shift(args[1:])
	case "trap":
		return r.trap(args[1:])
	case "umask":
		return r.umask(args[1:])
	case "wait":
		return r.wait(ctx, args[1:])
	}
	// Before applet lookup and before PATH: a builtin this shell recognises and
	// does not implement must say so, rather than depend on whether something
	// of that name happens to be installed.
	if status, refused := r.reportUnimplementedBuiltin(args[0]); refused {
		return status
	}
	applet, ok := r.lookupApplet(args[0])
	if !ok {
		return r.runExternal(ctx, args)
	}
	err := applet.Run(applets.WithProcessView(ctx, r), args[1:], r.streams.Stdin, r.streams.Stdout, r.streams.Stderr)
	if err == nil {
		return 0
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return contextStatus(ctx)
	}
	if errors.Is(err, errPipelineDownstreamClosed) {
		return 0
	}
	status, message := AppletFailure(args[0], err)
	if message != "" {
		fmt.Fprintln(r.streams.Stderr, message)
	}
	return status
}

// AppletFailure turns an applet's error into the status and the one-line
// diagnostic that go with it. Exported because `nemosh cat missing` has to fail
// exactly the way `cat missing` inside the shell does, and the CLI cannot reach
// the shell's copy of this. It used to have no copy at all: direct dispatch
// dropped the applet-name prefix, and printed nothing whatever when the failure
// carried its own status, so `nemosh env python3` exited 127 in silence.
//
// An empty message means there is nothing to print: a bare applets.ExitStatus
// carries a status without a diagnostic, and so does ErrExitFalse.
func AppletFailure(name string, err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	if status, ok := applets.StatusCode(err); ok {
		if message, ok := applets.StatusMessage(err); ok {
			return status, name + ": " + message
		}
		return status, ""
	}
	if errors.Is(err, applets.ErrExitFalse) {
		return 1, ""
	}
	return 1, fmt.Sprintf("%s: %v", name, err)
}

func exitStatus(args []string, savedStatus int) int {
	if len(args) == 0 {
		return savedStatus
	}
	status, err := strconv.Atoi(args[0])
	if err != nil {
		return 2
	}
	return status & 0xff
}
