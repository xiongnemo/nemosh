package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (r Runtime) dot(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, ".: missing file")
		return 2
	}
	resolved, err := r.ResolveNemoshPath(args[0])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, ".: %s: %v\n", args[0], err)
		return 1
	}
	if resolved.Device {
		fmt.Fprintf(r.streams.Stderr, ".: %s: not a regular file\n", args[0])
		return 1
	}
	data, err := os.ReadFile(resolved.Native)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, ".: %s: %v\n", args[0], err)
		return 1
	}
	child := r
	child.sourceDepth++
	status, control := child.runScriptResult(ctx, string(data), false)
	if control == flowReturn {
		return status
	}
	return status
}

func (r Runtime) eval(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	return r.runScript(ctx, strings.Join(args, " "), false)
}
