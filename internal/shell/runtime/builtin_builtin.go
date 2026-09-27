package runtime

import (
	"context"
	"fmt"
)

// builtinBuiltin is bash's `builtin name [args]`: the builtin called name, even where a
// function has taken the name, which is what a wrapper such as `cd() { builtin cd "$@"; }`
// needs. It was "builtin: not found", so every such wrapper failed. An applet runs inside the
// shell and counts, as `type` calls it a builtin; a program on PATH does not. busybox has not
// got it.
func (r Runtime) builtinBuiltin(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	if _, applet := r.lookupApplet(args[0]); !applet && !isRuntimeBuiltin(args[0]) {
		fmt.Fprintf(r.streams.Stderr, "builtin: %s: not a shell builtin\n", args[0])
		return 1
	}
	return r.runCommandResolved(ctx, args, false)
}
