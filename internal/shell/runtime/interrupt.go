package runtime

import (
	"context"
	"errors"
)

var errShellInterrupt = errors.New("shell interrupt")

func InterruptContext(parent context.Context) (context.Context, func()) {
	ctx, interrupt, _ := InterruptContextWithRelease(parent)
	return ctx, interrupt
}

func InterruptContextWithRelease(parent context.Context) (context.Context, func(), func()) {
	ctx, cancel := context.WithCancelCause(parent)
	return ctx, func() { cancel(errShellInterrupt) }, func() { cancel(context.Canceled) }
}

func contextStatus(ctx context.Context) int {
	if isShellInterrupt(ctx) {
		return 130
	}
	return 1
}

func isShellInterrupt(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errShellInterrupt)
}

func IsShellInterrupt(ctx context.Context) bool {
	return isShellInterrupt(ctx)
}
