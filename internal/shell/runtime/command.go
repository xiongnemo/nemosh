package runtime

import (
	"context"
	"fmt"
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

func (r Runtime) commandV(args []string) int {
	if len(args) == 0 {
		return 0
	}
	name := args[0]
	if r.isKnownCommand(name) {
		fmt.Fprintln(r.streams.Stdout, name)
		return 0
	}
	return 1
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
	_, ok := r.registry.Lookup(name)
	return ok
}

func isRuntimeBuiltin(name string) bool {
	switch name {
	case ".", "break", "cd", "command", "continue", "eval", "exec", "exit", "export", "getopts", "jobs", "pwd", "read", "readonly", "set", "shift", "source", "trap", "umask", "unset", "wait":
		return true
	default:
		return false
	}
}

func isSpecialBuiltin(name string) bool {
	switch name {
	case ".", "break", "continue", "eval", "exec", "exit", "export", "readonly", "return", "set", "shift", "source", "trap", "unset":
		return true
	default:
		return false
	}
}
