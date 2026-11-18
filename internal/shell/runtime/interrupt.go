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

// ConsoleInterruptContextWithRelease is InterruptContextWithRelease for an interrupt that is the
// console's Ctrl-C, which the program in the foreground was sent as well; prompt says the
// command was typed at the prompt. See external_cancel.go for what that changes.
func ConsoleInterruptContextWithRelease(parent context.Context, prompt bool) (context.Context, func(), func()) {
	ctx, cancel := context.WithCancelCause(parent)
	return ctx, func() { cancel(consoleInterrupt{prompt: prompt}) }, func() { cancel(context.Canceled) }
}

// consoleInterrupt is the shell's interrupt as the console delivered it, to the shell and to
// every program on the console. It is errShellInterrupt to everything that asks.
type consoleInterrupt struct{ prompt bool }

func (consoleInterrupt) Error() string { return errShellInterrupt.Error() }
func (consoleInterrupt) Unwrap() error { return errShellInterrupt }

func contextStatus(ctx context.Context) int {
	if isShellInterrupt(ctx) {
		return 130
	}
	if signal, ok := errors.AsType[jobSignal](context.Cause(ctx)); ok {
		return 128 + int(signal)
	}
	return 1
}

// endedBySignal is whether ctx ended by the shell's interrupt or by a signal sent to its job.
func endedBySignal(ctx context.Context) bool {
	if isShellInterrupt(ctx) {
		return true
	}
	_, signalled := errors.AsType[jobSignal](context.Cause(ctx))
	return signalled
}

func isShellInterrupt(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errShellInterrupt)
}

func IsShellInterrupt(ctx context.Context) bool {
	return isShellInterrupt(ctx)
}
