package runtime

import (
	"context"
	"fmt"
	"path/filepath"
)

func (r Runtime) command(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	if args[0] == "-v" {
		return r.commandV(args[1:])
	}
	return r.runCommandResolved(ctx, args, false)
}

// commandV answers POSIX's `command -v`: a builtin, function, or applet is
// reported by name, and anything else by the pathname the shell would launch.
// The pathname half was missing, so `command -v git` said nothing and exited 1
// while plain `git` ran -- the two disagreeing about what the shell can do.
func (r Runtime) commandV(args []string) int {
	if len(args) == 0 {
		return 0
	}
	name := args[0]
	if r.isKnownCommand(name) {
		fmt.Fprintln(r.streams.Stdout, name)
		return 0
	}
	// The same lookup dispatch uses, so the two cannot drift apart again.
	resolved, err := r.externalCommandPath(name)
	if err != nil {
		return 1
	}
	fmt.Fprintln(r.streams.Stdout, filepath.ToSlash(resolved))
	return 0
}

func (r Runtime) isKnownCommand(name string) bool {
	if isRuntimeBuiltin(name) {
		return true
	}
	if parsed, ok := newFunctionName(name); ok {
		if _, found := r.functions[parsed]; found {
			return true
		}
	}
	_, ok := r.lookupApplet(name)
	return ok
}

func isRuntimeBuiltin(name string) bool {
	switch name {
	// `return` belongs here even though it is dispatched from
	// controlFlowBuiltin rather than the switch in runCommandResolved: this
	// answers `command -v` and `type`, and leaving it out had them report a
	// builtin that plainly works as absent.
	case ":", ".", "alias", "break", "cd", "command", "continue", "eval", "exec", "exit", "export", "getopts", "jobs", "let", "local", "pwd", "read", "readonly", "return", "set", "shift", "source", "times", "trap", "type", "umask", "unalias", "unset", "wait":
		return true
	default:
		return false
	}
}

func isSpecialBuiltin(name string) bool {
	switch name {
	case ":", ".", "break", "continue", "eval", "exec", "exit", "export", "readonly", "return", "set", "shift", "source", "trap", "unset":
		return true
	default:
		return false
	}
}
