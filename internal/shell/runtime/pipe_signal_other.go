//go:build !windows

package runtime

import (
	"os"
	"os/signal"
	"syscall"
)

// pipeSink takes the SIGPIPE a caught disposition has Go deliver, and nothing reads it: the
// write that raised it has already told the shell its reader has gone (pipe_stage.go). What
// the Notify buys is that a write to standard output no reader is left for fails, instead of
// ending the process before the shell can run its trap.
var pipeSink = make(chan os.Signal, 1)

// setProcessPipe makes the process's SIGPIPE what the shell's trap made it. Ignored, the
// programs the shell starts inherit it, as they do a shell's; caught, they start with the
// default, as a caught signal is in a new program.
func setProcessPipe(disposition pipeDisposition) {
	switch disposition {
	case pipeIgnored:
		signal.Ignore(syscall.SIGPIPE)
	case pipeCaught:
		signal.Notify(pipeSink, syscall.SIGPIPE)
	default:
		// Notify first: Reset undoes only a Notify, and SIGPIPE ignored would stay ignored.
		signal.Notify(pipeSink, syscall.SIGPIPE)
		signal.Reset(syscall.SIGPIPE)
	}
}
