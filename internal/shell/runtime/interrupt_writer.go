package runtime

import (
	"context"
	"io"
	"os"
)

// interruptibleWriter fails once the command's context is cancelled, which is how Ctrl-C
// reaches an applet that is not watching for it.
//
// Almost every applet is a simpleApplet built with `run:`, and that signature has no
// context in it at all: simpleApplet.Run checks cancellation once, before calling, and
// after that a loop producing output cannot see it. `seq 1 100000000` was measured
// running for 8.9 seconds and writing 888 MB after the interrupt, then reporting 130 as
// though it had stopped. Passing a context to all 130 applets would fix the ones edited
// today and rely on every applet written later remembering; failing the write fixes them
// at the one place they are dispatched, and an applet that ignores the error was already
// broken for `producer | head -1`.
//
// The error is deliberately the context's own, because runCommand answers
// contextStatus(ctx) -- 130 for an interrupt -- and says nothing when the error it gets
// back is ctx.Err(). Anything else would be reported, and `bc: context canceled` reaching
// a terminal is what that looks like: a Go error string shown to someone who pressed
// Ctrl-C.
//
// It holds the context rather than ctx.Err, which as a method value was an allocation of
// its own. The shell's call frame keeps one and hands out a pointer to it (applet_frame.go);
// a context that can never be cancelled gets no wrapper at all, and costs nothing per write.
type interruptibleWriter struct {
	writer io.Writer
	done   <-chan struct{}
	ctx    context.Context
}

func (w *interruptibleWriter) Write(p []byte) (int, error) {
	select {
	case <-w.done:
		return 0, w.ctx.Err()
	default:
	}
	return w.writer.Write(p)
}

// TerminalFile passes through the question of which file this stream ends at.
//
// Without this, wrapping stdout would silently undo what fd_stream.go exists for: `ls`
// would stop laying out columns on a terminal and `--color=auto` would stop colouring,
// because both ask the stream to name its file and a wrapper that does not answer looks
// exactly like a pipe.
func (w *interruptibleWriter) TerminalFile() *os.File { return terminalFileOf(w.writer) }
