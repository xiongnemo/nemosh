package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// errShellInterrupt is the cause of the shell's interrupt, ^C. It is applets.ErrInterrupt, so
// an applet told to ignore SIGINT, tee -i, can tell it from the rest.
var errShellInterrupt = fmt.Errorf("shell interrupt: %w", applets.ErrInterrupt)

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
	if signal, ok := errors.AsType[jobSignal](context.Cause(ctx)); ok {
		return 128 + int(signal)
	}
	return 1
}

func isShellInterrupt(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errShellInterrupt)
}

func IsShellInterrupt(ctx context.Context) bool {
	return isShellInterrupt(ctx)
}
