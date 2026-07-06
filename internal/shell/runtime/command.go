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
	return r.runCommand(ctx, args)
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
	_, ok := r.registry.Lookup(name)
	return ok
}

func isRuntimeBuiltin(name string) bool {
	switch name {
	case ".", "break", "cd", "command", "continue", "eval", "exec", "exit", "export", "pwd", "read", "readonly", "set", "shift", "trap", "umask", "unset", "wait":
		return true
	default:
		return false
	}
}
