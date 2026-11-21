package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Runtime is the shell as one command sees it. It is passed by value, and a method that changes
// what only its own command should see -- a redirection's streams, a function call's parameters
// and frame, the flags below that mark where an expansion is -- changes its copy. Every method
// call copies it, so its size is the interpreter's cost: 512 bytes more made a loop a third
// slower, measured. What a copy shares with the value it was copied from, the tables a command
// reads and writes through, is therefore one pointer, shellState, and not on the value at all.
type Runtime struct {
	*shellState
	streams Streams
	fds     *fdTable
	// trapStatus is the status the running trap's action was entered with, which an `exit`
	// given none ends with; see runTrap. Nil outside one.
	trapStatus *int
	params     *parameters
	// aliasChain is the aliases whose text is running, which are not substituted again inside
	// it; see alias_expand.go. aliasView is the aliases the running line was read with, and
	// nil for the ones defined now; see alias_view.go.
	aliasChain []string
	aliasView  map[string]string
	// shadowed is what a command's temporary assignments took out of arrays; see array_shadow.go.
	shadowed []shadowedArray
	// locals belongs to the function call in progress and is nil outside one,
	// which is how `local` knows there is nothing to restore to.
	locals *localScope
	// frames is the call stack, innermost first, and scriptFile the file at the bottom
	// of it: $FUNCNAME, $BASH_SOURCE, $BASH_LINENO and `caller`. See call_stack.go.
	frames     *callFrame
	scriptFile string
	// traceTurn orders a pipeline's `set -x` lines, a stage's after the stage's before it; see
	// trace_turn.go. Shared with what the stage clones, a subshell or a nested pipeline in it.
	traceTurn *traceTurn
	// pipeStage is the pipeline stage whose pipe this runtime's output goes into, which a
	// write that finds its reader gone ends; see pipe_stage.go. Shared as traceTurn is.
	pipeStage *pipeStage
	// errExitSuppressed marks the places POSIX 2.9.1 exempts from `set -e`: a
	// condition, a negated pipeline, and every command but the last of an
	// and-or list. It rides on the Runtime value rather than the shared options
	// pointer so entering one of those places cannot leak out of it.
	// noFieldSplit marks an expansion whose result POSIX 2.6.5 exempts from field
	// splitting: the value of an assignment. On the Runtime value like
	// errExitSuppressed, so it cannot leak past the word it was set for.
	noFieldSplit      bool
	errExitSuppressed bool
	// jobTop marks a background job's own commands, the outermost pipelines of what it runs,
	// whose failure runs no ERR trap in it; see jobTraps. On the value, and dropped by the
	// first pipeline it reaches, so the commands inside those run theirs.
	jobTop bool
	// plainAssignment marks the writes of an assignment statement, which abandon the command
	// when they cannot be made; see failAssignment. On the value for the same reason.
	plainAssignment bool
	// arithmeticCommand marks the `let` an `((expr))` became; see arithmeticCommandTokens. On
	// the value, so it is that command's alone.
	arithmeticCommand bool
	// arrayOperands is the running command's operands written as array literals; see
	// arrayLiteralOperands. On the value, so it is that command's alone.
	arrayOperands map[string]bool
	// operandQuoted marks a `${...}` inside double quotes, which changes what a single quote
	// in its operator's word means; see operand_quoting.go. On the value for the same reason.
	operandQuoted bool
	// starFields marks an unquoted `*` list, `${a[*]}`, which is a field per element as
	// unquoted `$*` is; see buildParameter. On the value, so it is that word's alone.
	starFields    bool
	sourceDepth   int
	functionDepth int
	// subshellDepth is $BASH_SUBSHELL; see subshellSnapshot.
	subshellDepth int
	// searchPath, when set, is where a command is looked for instead of PATH: `command -p`;
	// see command_options.go.
	searchPath  string
	interactive interactiveState
}

type shellLifecycle struct {
	exitSuppressed bool
}

type trapName string

const (
	trapExit trapName = "EXIT"
	trapINT  trapName = "INT"
	// trapERR runs after a command fails where `set -e` would exit; see errTrapTriggers.
	trapERR trapName = "ERR"
	// trapRETURN runs as a function returns or a sourced file finishes; see return_trap.go.
	trapRETURN trapName = "RETURN"
	// trapDEBUG runs before each command; see debug_trap.go.
	trapDEBUG trapName = "DEBUG"
	// The signals a trap can catch besides INT and PIPE; see signal_inbox.go.
	trapHUP  trapName = "HUP"
	trapQUIT trapName = "QUIT"
	trapILL  trapName = "ILL"
	trapFPE  trapName = "FPE"
	trapSEGV trapName = "SEGV"
	trapTERM trapName = "TERM"
	trapABRT trapName = "ABRT"
)

func New(registry applets.Registry, streams Streams) Runtime {
	return NewWithState(registry, streams, hostState())
}

func NewRuntime(registry applets.Registry, streams Streams) (Runtime, error) {
	return NewRuntimeWithState(registry, streams, hostState())
}

func (r Runtime) runCommandWithRedirects(ctx context.Context, args []string) int {
	commandArgs, streams, cleanup, err := r.applyRedirects(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1
	}
	commandRuntime, err := r.snapshotShared()
	if err != nil {
		cleanupErr := cleanup()
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), errors.Join(err, cleanupErr))
		return 1
	}
	commandRuntime, err = commandRuntime.withStreams(streams)
	if err != nil {
		cleanupErr := cleanup()
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), errors.Join(err, cleanupErr))
		return 1
	}
	status := commandRuntime.runCommand(ctx, commandArgs)
	commandRuntime.jobScope.drain()
	closeErr := commandRuntime.fds.closeAll()
	if err := cleanup(); err != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1
	}
	if closeErr != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), closeErr)
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
	args, allowFunctions = r.unixPathCommand(args, allowFunctions)
	if allowFunctions && !isSpecialBuiltin(args[0]) {
		if definition, found := r.calledFunction(args[0]); found {
			return r.callFunction(ctx, definition, args[1:])
		}
	}
	return r.containedError(args[0], r.runBuiltinOrProgram(ctx, args))
}

func (r Runtime) runBuiltinOrProgram(ctx context.Context, args []string) int {
	if status, answered := r.builtinHelp(args); answered {
		return status
	}
	switch args[0] {
	case "alias":
		return r.alias(args[1:])
	case "unalias":
		return r.unalias(args[1:])
	case "compgen", "complete", "compopt":
		return r.completionBuiltin(ctx, args)
	case "bind":
		return r.bindBuiltin(args[1:])
	case "_get_comp_words_by_ref", "_init_completion", "_filedir", "__ltrim_colon_completions":
		return r.completionHelper(args)
	case "local":
		return r.local(ctx, args[1:])
	case "type":
		return r.typeBuiltin(args[1:])
	case "let":
		return r.let(args[1:])
	case "times":
		return r.times()
	case "time":
		return r.timeBuiltin(ctx, args[1:])
	case "timeout":
		return r.timeoutBuiltin(ctx, args[1:])
	case "flock":
		return r.flockBuiltin(ctx, args[1:])
	case "watch":
		return r.watchBuiltin(ctx, args[1:])
	case "help":
		return r.help(args[1:])
	case "history":
		return r.historyBuiltin(args[1:])
	case ":":
		// The null command of POSIX 2.14: its arguments are expanded, which has
		// already happened by the time it gets here, and it returns zero.
		// `while :; do ... done` is the ordinary way to write an endless loop,
		// and without this it was a failed lookup for a program named `:`.
		return 0
	case ".", "source":
		return r.dot(ctx, args)
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
	case "disown", "enable":
		return r.bashOnlyBuiltin(args)
	case "which":
		return r.whichBuiltin(args[1:])
	case "kill":
		return r.killBuiltin(args[1:])
	case "export":
		return r.export(args[1:])
	case "unset":
		return r.unset(ctx, args[1:])
	case "mapfile", "readarray":
		return r.mapfile(ctx, args[0], args[1:])
	case "pushd", "popd", "dirs":
		return r.directoryStackBuiltin(args)
	case "title":
		return r.titleBuiltin(args[1:])
	case "pwd":
		return r.pwdBuiltin(args[1:])
	case "shopt":
		return r.shoptBuiltin(args[1:])
	case "declare", "typeset":
		return r.declareBuiltin(ctx, args[1:])
	case "caller":
		return r.caller(args[1:])
	case "read":
		return r.read(ctx, args[1:])
	case "readonly":
		return r.readonlyBuiltin(ctx, args[1:])
	case "set":
		return r.set(args[1:])
	case "builtin":
		return r.builtinBuiltin(ctx, args[1:])
	case "hash":
		return r.hashBuiltin(args[1:])
	case "shift":
		return r.shift(args[1:])
	case "trap":
		return r.trap(args[1:])
	case "umask":
		return r.umask(args[1:])
	case "wait":
		return r.wait(ctx, args[1:])
	case "printf":
		if len(args) > 1 && args[1] == "-v" {
			return r.printfToVariable(ctx, args[2:])
		}
	}
	// Before applet lookup and before PATH: a builtin this shell recognises and
	// does not implement must say so, rather than depend on whether something
	// of that name happens to be installed.
	if status, refused := r.reportUnimplementedBuiltin(args[0]); refused {
		return status
	}
	r.startingProgram(args[0])
	// A program's write into a pipe no one reads ends the program, and the shell goes on; a
	// builtin's ends the shell's stage. See pipe_stage.go.
	if !ashBuiltinApplets(args[0]) {
		defer r.pipeStage.running()()
	}
	applet, ok := r.lookupApplet(args[0])
	if !ok {
		return r.runExternal(ctx, args)
	}
	return r.runApplet(ctx, applet, r.xpgEchoArgs(args))
}
