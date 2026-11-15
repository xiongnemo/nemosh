package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An applet is handed more than its arguments: a context that answers for the shell -- its
// ProcessView and its ProgramRunner -- a stdin whose reads end when the command is cancelled,
// and a stdout whose writes do. Built per call, those were seven allocations every time a
// command ran an applet: two context values, two copies of the whole Runtime into the
// interfaces they hold, the two stream wrappers, and a method value for the writer's error.
// A loop of two thousand commands made fourteen thousand of them.
//
// An appletFrame is all of that in one object: the context is the frame, and the reader and
// writer are pointers into it, which interfaces hold without allocating. A frame is never
// changed once made, and the shell keeps its last few (appletFrames) and hands an applet one
// of those again when the call is the same -- the same context, and a Runtime identical to
// the byte -- which is what a command repeated in a loop is. So a repeated command costs no
// allocation, and any other costs one, the frame.
//
// Reusing one mutable frame per call, overwritten each time, was the first design and was
// wrong: an applet's context and streams outlive it more often than it looks -- the editor
// and less leave a goroutine waiting on ctx.Done(), nc returns with its copy goroutines still
// reading, net/http's transport holds a request's context a moment after the body is read --
// and each of those would have read a frame while the next call rewrote it, which for an
// interface is a torn read rather than merely stale data. A frame that is never changed is
// safe to hold for as long as anything likes. NEMOSH_APPLET_FRAMES=fresh makes every call
// build its own, which is the way back if the matching is ever suspected.
type appletFrame struct {
	parent context.Context
	shell  Runtime
	in     applets.ContextReader
	out    interruptibleWriter
	// wraps is whether stdout is wrapped: a context that can never be cancelled leaves it
	// as it is, as it costs nothing per write that way.
	wraps bool
}

// runApplet runs an applet the shell found for args[0]. The frame is the context, which
// answers for the shell as ProcessView and as the runner env, xargs, find and awk run a
// program no applet has with (external_program.go), and it holds the two streams. Stdout is
// wrapped so that Ctrl-C reaches an applet that is not watching the context -- which is
// almost all of them; see interrupt_writer.go. Stderr is left alone: a diagnostic written
// on the way out is still worth seeing.
func (r Runtime) runApplet(ctx context.Context, applet applets.Applet, args []string) int {
	frame := r.appletFrames.frameFor(ctx, &r)
	stdin, stdout := frame.streams()
	err := applet.Run(frame, args[1:], stdin, stdout, r.streams.Stderr)
	return r.appletStatus(ctx, args[0], err)
}

// appletFrames is the frames recently made, shared by every snapshot of one shell. A few,
// because a background job or a pipeline's stage calls in its own context alongside the
// foreground; looked up and replaced atomically, since those run on goroutines of their own.
type appletFrames struct {
	recent [4]atomic.Pointer[appletFrame]
	next   atomic.Uint32
}

// newAppletFrames is the shell's frames, or none under NEMOSH_APPLET_FRAMES=fresh.
func newAppletFrames() *appletFrames {
	if os.Getenv("NEMOSH_APPLET_FRAMES") == "fresh" {
		return nil
	}
	return &appletFrames{}
}

// frameFor is the frame for an applet called in ctx by r: a recent one made for the same
// call, or a new one, which then becomes recent.
func (c *appletFrames) frameFor(ctx context.Context, r *Runtime) *appletFrame {
	if c != nil {
		for index := range c.recent {
			if frame := c.recent[index].Load(); frame != nil && frame.madeFor(ctx, r) {
				return frame
			}
		}
	}
	frame := newAppletFrame(ctx, *r)
	if c != nil {
		c.recent[c.next.Add(1)%uint32(len(c.recent))].Store(frame)
	}
	return frame
}

func newAppletFrame(ctx context.Context, r Runtime) *appletFrame {
	frame := &appletFrame{parent: ctx, shell: r}
	frame.in = applets.ContextReader{Ctx: frame, Reader: r.streams.Stdin}
	if done := ctx.Done(); done != nil {
		frame.out = interruptibleWriter{writer: r.streams.Stdout, done: done, ctx: ctx}
		frame.wraps = true
	}
	return frame
}

// streams is the stdin and stdout the applet is handed.
func (f *appletFrame) streams() (io.Reader, io.Writer) {
	if f.wraps {
		return &f.in, &f.out
	}
	return &f.in, f.shell.streams.Stdout
}

// madeFor reports whether the frame is the one a call in ctx by r would make. The Runtime
// is compared as memory: equal bytes are the same pointers, maps and strings, so the frame
// answers exactly as a new one would, and a field added to Runtime later is compared
// without anyone remembering to. Two Runtimes that differ only in, say, a string's backing
// array compare unequal, which costs a frame and is never wrong.
func (f *appletFrame) madeFor(ctx context.Context, r *Runtime) bool {
	if !applets.SameContext(f.parent, ctx) {
		return false
	}
	size := unsafe.Sizeof(*r)
	return bytes.Equal(unsafe.Slice((*byte)(unsafe.Pointer(&f.shell)), size), unsafe.Slice((*byte)(unsafe.Pointer(r)), size))
}

func (f *appletFrame) Deadline() (time.Time, bool) { return f.parent.Deadline() }
func (f *appletFrame) Done() <-chan struct{}       { return f.parent.Done() }
func (f *appletFrame) Err() error                  { return f.parent.Err() }

// Value answers the shell's two keys with the frame's Runtime, and passes every other to
// the context the call was made in -- the context package's own lookup among them, which is
// how a context an applet derives from this one is cancelled with the call's.
func (f *appletFrame) Value(key any) any {
	if applets.IsShellKey(key) {
		return &f.shell
	}
	return f.parent.Value(key)
}

// isShellView reports whether view is the shell itself, by value as WithProcessView passes
// it or by pointer as a frame does, rather than one an applet made -- env's, with a PATH of
// its own.
func isShellView(view applets.ProcessView) bool {
	switch view.(type) {
	case Runtime, *Runtime:
		return true
	}
	return false
}
