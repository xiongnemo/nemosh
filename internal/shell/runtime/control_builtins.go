package runtime

import (
	"context"
	"fmt"
	"slices"
	"strconv"
)

// controlFlowBuiltin runs the builtins that answer with a control transfer
// rather than only a status. Each of them is a POSIX special builtin, so its
// leading assignments persist after it completes (2.9.1) and are applied here
// before the transfer leaves.
func (r Runtime) controlFlowBuiltin(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation, savedStatus int) (lineResult, bool) {
	named := throughCommandPrefix(args)
	prefixed := len(named) != len(args)
	// Under command, eval and . are plain builtins, whose errors end only themselves; see
	// builtin_error_contained.go.
	plain := slices.Contains(args[:len(args)-len(named)], "command")
	args = named
	switch args[0] {
	case "exit", "exec", "return", "break", "continue", "eval", ".", "source":
	case "fc":
		return r.fcCommand(ctx, args[1:], assignments, operations, savedStatus), true
	default:
		return lineResult{}, false
	}
	if status := r.assignVars(assignments); status != 0 {
		return lineResult{status: status}, true
	}
	switch args[0] {
	case "eval":
		return r.plainResult(plain, r.withAppliedRedirectsFor(!plain, operations, func(redirected Runtime) lineResult {
			return redirected.evalResult(ctx, args[1:], savedStatus)
		})), true
	case ".", "source":
		return r.plainResult(plain, r.withAppliedRedirectsFor(!plain, operations, func(redirected Runtime) lineResult {
			return redirected.dotResult(ctx, args, savedStatus, !prefixed)
		})), true
	case "exit", "return", "break", "continue":
		// Their redirections are made as any command's are, though nothing is written through
		// them but a diagnostic: `break > log` creates log in both references. They were not
		// made at all.
		return r.withAppliedRedirectsFor(true, operations, func(redirected Runtime) lineResult {
			return redirected.transferControl(args, savedStatus)
		}), true
	case "exec":
		// `--` ends exec's options, as in both references: `exec -- 3>&1` is exec with only
		// redirections, and `exec -- echo hi` runs echo. It ran a command called --.
		command := args[1:]
		if len(command) > 0 && command[0] == "--" {
			command = command[1:]
		}
		if len(command) == 0 {
			return lineResult{status: r.execRedirect(operations)}, true
		}
		// A prefix assignment is in the environment of the command exec runs, as it is for
		// any command: `pre=x exec printenv pre` says x in both references. It was made in
		// the shell alone, above, and the command never saw it.
		runner := r
		if temporary := r.withLocalAssignments(assignments); temporary != nil {
			runner = *temporary
		}
		return lineResult{status: runner.execBuiltin(ctx, command), control: flowExec}, true
	default:
		return lineResult{control: flowContinue}, true
	}
}

// transferControl is exit, return, break or continue, once their redirections are made.
func (r Runtime) transferControl(args []string, savedStatus int) lineResult {
	switch args[0] {
	case "exit":
		if len(args) == 1 && r.trapStatus != nil {
			savedStatus = *r.trapStatus
		}
		r.badStatus(args)
		return lineResult{status: exitStatus(args[1:], savedStatus), control: flowExit}
	case "return":
		status := exitStatus(args[1:], savedStatus)
		// A status that is no number ends the script, as busybox's number() raises it; it
		// went on with 2.
		if r.badStatus(args) {
			return lineResult{status: 2, control: flowAbort}
		}
		if r.sourceDepth == 0 && r.functionDepth == 0 {
			// Outside a function and a sourced file, return ends the shell as exit does, as
			// busybox-w32 reads it: `return 3` ends a script with 3, the EXIT trap seeing 3, and
			// ends the subshell or $(...) it is in. It was reported, and the script went on.
			// At a prompt, and in a trap's action, it is still only reported, as bash reports
			// it everywhere: `trap 'return 42' DEBUG` is bash's alone, and bash goes on.
			if r.interactive.session && r.subshellDepth == 0 || len(r.trapRunning) > 0 {
				fmt.Fprintln(r.streams.Stderr, "return: not in a sourced script")
				return lineResult{status: status}
			}
			return lineResult{status: status, control: flowExit}
		}
		return lineResult{status: status, control: flowReturn}
	}
	return r.loopControlResult(args[0], args[1:])
}

// throughCommandPrefix is the command a bare `command` or `builtin` in front names, when that
// is one of the builtins that transfer control: `command continue` continues, as it does in
// both references, and `builtin break` breaks, as it does in bash. Run as ordinary builtins,
// their control was lost and only a status came back.
func throughCommandPrefix(args []string) []string {
	rest := args
	for len(rest) > 1 && (rest[0] == "command" || rest[0] == "builtin") {
		rest = rest[1:]
	}
	switch rest[0] {
	case "exit", "exec", "return", "break", "continue", "eval", ".", "source":
		return rest
	}
	return args
}

// badStatus is whether exit or return was given a status that is no number, which it reports
// as "Illegal number", as every ash does and as shift, break, continue and wait do here too. It
// said nothing, and then bash's "x: numeric argument required".
func (r Runtime) badStatus(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if _, err := strconv.Atoi(args[1]); err == nil {
		return false
	}
	fmt.Fprintf(r.streams.Stderr, "%s: Illegal number: %s\n", args[0], args[1])
	return true
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
