package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) execBuiltin(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	return r.runCommand(ctx, args)
}

// execRedirect is `exec` with redirections and no command: POSIX 2.14 says the
// redirections apply to the shell itself and stay in force for the rest of the
// script. Nemosh dropped them on the floor, so `exec > log` reported success,
// created no file, and left output going where it always had.
//
// The shell's own table is rebound in place rather than cloned, because the
// streams are descriptor views that read the table at write time -- rebinding
// fd 1 here is exactly what makes every later command follow it.
func (r Runtime) execRedirect(operations []redirectOperation) int {
	if len(operations) == 0 {
		return 0
	}
	if err := r.applyRedirectOperations(r.fds, operations); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	return 0
}
