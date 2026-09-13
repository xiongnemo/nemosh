package runtime_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **Ctrl-C stops a command that is producing output.**
//
// Reported from a real terminal: `seq 1 100000000` could not be interrupted. Measured
// before the fix -- the interrupt was delivered after 200ms and seq ran for 8.9 seconds
// and wrote 888 MB anyway, then reported 130 as though it had been stopped.
//
// The cause is that most applets are simpleApplet values built with `run:`, whose
// signature has no context at all: simpleApplet.Run checks cancellation once, before
// calling it, and after that nothing inside a loop can see it. seq is one, and so are
// bc, yes and every other producer. Handing each of them a context is 130 edits and
// relies on every future applet remembering; making the write fail is one edit at the
// single place applets are dispatched, and every applet that propagates a write error
// -- which seq already did, for `producer | head -1` -- stops on its own.
//
// The interrupt is fired from inside the writer rather than after a delay, so this
// cannot pass by winning a race: the cancellation is complete before the write that
// follows it. See AGENTS.md on tests that assert on background work.

// interruptingWriter fires once the applet has produced something, then counts what
// arrives after that -- which is the thing being measured.
type interruptingWriter struct {
	fire  func()
	once  sync.Once
	total atomic.Int64
}

func (w *interruptingWriter) Write(p []byte) (int, error) {
	if w.total.Add(int64(len(p))) > 1024 {
		w.once.Do(w.fire)
	}
	return len(p), nil
}

// TerminalFile keeps this writer honest about the convention in fd_stream.go: a stream
// names the file it ends at, and this one ends at nothing.
func (w *interruptingWriter) TerminalFile() *os.File { return nil }

func TestRuntime_interruptStopsAProducingApplet(t *testing.T) {
	ctx, interrupt, release := runtime.InterruptContextWithRelease(context.Background())
	defer release()
	out := &interruptingWriter{fire: interrupt}
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: out, Stderr: &stderr})

	script, err := runtime.ParseScript("seq 1 100000000\n")
	if err != nil {
		t.Fatal(err)
	}
	status := rt.RunInteractive(ctx, script).Status

	// A byte count rather than a duration: how much work happened after the interrupt is
	// the question, and it does not change with how fast the machine is.
	if written := out.total.Load(); written > 1<<20 {
		t.Errorf("seq wrote %d bytes after being interrupted at 1 KiB, so the interrupt did "+
			"not reach it; an applet built with simpleApplet's `run:` never sees the context, "+
			"so the write is what has to fail", written)
	}
	if status != 130 {
		t.Errorf("status = %d, want 130", status)
	}
	// An interrupt is not a diagnostic. `bc: context canceled` reached a real terminal
	// this way, which is a Go error string leaking to a person who pressed Ctrl-C.
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing: being interrupted is not a failure to report", stderr.String())
	}
}

// firingWriter runs fire once, as soon as the applet has written anything, and keeps what
// arrived so the test can say where the output stopped.
type firingWriter struct {
	fire func()
	once sync.Once
	seen bytes.Buffer
	mu   sync.Mutex
}

func (w *firingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.seen.Write(p)
	w.mu.Unlock()
	w.once.Do(w.fire)
	return len(p), nil
}

func (w *firingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seen.String()
}

func (w *firingWriter) TerminalFile() *os.File { return nil }

// TestRuntime_interruptInsideAnAppletIsNotADiagnostic covers Ctrl-C inside `bc`.
//
// Reported from a real terminal: interrupting bc answered `bc: context canceled`, a Go
// error string shown to someone who pressed a key. The status was already 130; it was
// only the message that was wrong.
//
// Every applet's stdin is wrapped by the registry in a contextReader so a long read can be
// cancelled, and a cancelled read comes back as context.Canceled. Every other applet in
// the tree *returns* what its scanner failed with -- runCommand answers 130 and says
// nothing for exactly that error -- and bc alone printed it and returned a plain non-zero
// instead. Nothing is lost for a real read failure: the shell prints `bc: <err>` from the
// returned error, which is the same sentence bc used to print itself.
//
// The interrupt is fired from inside the writer, so it has happened before the read that
// observes it; there is no sleep to lose a race with.
func TestRuntime_interruptInsideAnAppletIsNotADiagnostic(t *testing.T) {
	ctx, interrupt, release := runtime.InterruptContextWithRelease(context.Background())
	defer release()
	reader, writer := io.Pipe()
	t.Cleanup(func() { writer.Close() })
	out := &firingWriter{fire: interrupt}
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: reader, Stdout: out, Stderr: &stderr})

	go func() { io.WriteString(writer, "1+2\n") }()

	script, err := runtime.ParseScript("bc\n")
	if err != nil {
		t.Fatal(err)
	}
	status := rt.RunInteractive(ctx, script).Status

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing: Ctrl-C is not a failure to report", stderr.String())
	}
	if status != 130 {
		t.Errorf("status = %d, want 130", status)
	}
	// The answer typed before the interrupt still arrived, so this is not passing by
	// stopping bc before it ever ran.
	if got := out.String(); got != "3\n" {
		t.Errorf("stdout = %q, want the one answer that was asked for before the interrupt", got)
	}
}
