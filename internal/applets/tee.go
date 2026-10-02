package applets

import (
	"context"
	"errors"
	"io"
	"os"
)

// ErrInterrupt is the cause the shell ends a command's context with for an interrupt: the ^C
// typed at it, or SIGINT sent to its job. An applet told to ignore SIGINT, tee -i, goes on
// through one; see ignoringInterrupt.
var ErrInterrupt = errors.New("interrupt")

// ignoringInterrupt is ctx for an applet that ignores SIGINT: it ends when ctx does, unless an
// interrupt is what ended ctx.
func ignoringInterrupt(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(parent))
	// A parent already ended is seen now. AfterFunc would see it too, but on a goroutine of
	// its own, and a read could start in between and block on a pipe nothing ends: so it did
	// on a Linux runner, tee -i waiting ten minutes on a context its parent had ended.
	if cause := context.Cause(parent); cause != nil && !errors.Is(cause, ErrInterrupt) {
		cancel(cause)
	}
	stop := context.AfterFunc(parent, func() {
		if cause := context.Cause(parent); !errors.Is(cause, ErrInterrupt) {
			cancel(cause)
		}
	})
	return ctx, func() {
		stop()
		cancel(context.Canceled)
	}
}

// tee is busybox's (coreutils/tee.c): `tee [-ai] [FILE]...`, stdin copied to stdout and to each
// FILE, - being stdout again. -a appends rather than truncating, and -i goes on through an
// interrupt: `make | tee -i build.log` keeps all that make wrote before ^C stopped it. A FILE
// that cannot be opened is named and the rest are written, the status then 1.
//
// It stopped at such a FILE before copying anything, so what came down the pipe went nowhere,
// not to the other FILEs and not to stdout. - was a file of that name, and -i was refused.
func newTeeApplet() Applet {
	return simpleApplet{name: "tee", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "ia", "")
		if err != nil {
			return err
		}
		if options.has('i') {
			var stop context.CancelFunc
			ctx, stop = ignoringInterrupt(ctx)
			defer stop()
		}
		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if options.has('a') {
			flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		}
		writers := []io.Writer{stdout}
		view := ProcessViewFromContext(ctx)
		failed := false
		for _, path := range paths {
			if path == "-" {
				writers = append(writers, stdout)
				continue
			}
			file, err := openTeeFile(view, path, flags)
			if err != nil {
				failed = true
				if failure := operandFailure(path, err); !reportOperand(ctx, failure) {
					return failure
				}
				continue
			}
			defer file.Close()
			writers = append(writers, file)
		}
		if _, err := copyWithContext(ctx, io.MultiWriter(writers...), stdin); err != nil {
			return err
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// openTeeFile opens a FILE, a device among them: `tee /dev/stderr` is how a pipeline shows
// what passes through it.
func openTeeFile(view ProcessView, path string, flags int) (io.WriteCloser, error) {
	// 0666 through the umask, as busybox's fopen makes the file.
	return openProcessOutput(view, path, flags, createMode(view, 0o666))
}
