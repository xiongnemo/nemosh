package runtime

import "context"

func (r Runtime) execBuiltin(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	return r.runCommand(ctx, args)
}
