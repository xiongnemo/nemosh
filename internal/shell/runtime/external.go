package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

func (r Runtime) runExternal(ctx context.Context, args []string) int {
	cmd := exec.CommandContext(ctx, platformPath(args[0]), args[1:]...)
	cmd.Stdin = r.streams.Stdin
	cmd.Stdout = r.streams.Stdout
	cmd.Stderr = r.streams.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(r.streams.Stderr, "%s: not found\n", args[0])
		return 127
	}
	return 0
}
