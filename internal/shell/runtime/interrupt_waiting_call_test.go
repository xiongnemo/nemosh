package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
	"github.com/xiongnemo/nemosh/internal/testutil/stall"
)

// Ctrl-C ends a command waiting in a call Windows has not answered -- an open of a share that
// has stopped answering, which took 102 s to fail -- as busybox-w32's child is ended. The
// command here is an applet that opens a named pipe nothing is written to, by path as a file
// on a share is opened, and reads it plainly: nothing but the pin on the goroutine running it
// can end that read.
func stallApplet() applets.Applet {
	return backgroundApplet{name: "stall", run: func(_ context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		file, err := os.Open(filepath.FromSlash(args[0]))
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Read(make([]byte, 16))
		return err
	}}
}

func awaitStalled(t *testing.T, opened <-chan struct{}) {
	t.Helper()
	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("the command never opened the pipe")
	}
}

func TestRuntime_interruptEndsACallTheCommandWaitsIn(t *testing.T) {
	for _, test := range []struct {
		name, line string
		prompt     bool
	}{
		{name: "at the prompt", line: "stall '%s'", prompt: true},
		{name: "a stage at the prompt", line: ": | stall '%s'", prompt: true},
		{name: "in a script", line: "stall '%s'"},
		{name: "a stage in a script", line: ": | stall '%s'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: the command, waiting in its read
			path, opened := stall.Pipe(t)
			var stderr bytes.Buffer
			rt := runtime.New(applets.NewRegistry(stallApplet()), runtime.Streams{Stdout: io.Discard, Stderr: &stderr})
			ctx, interrupt := runtime.InterruptContext(context.Background())
			line := fmt.Sprintf(test.line, path) + "\n"
			statuses := make(chan int, 1)
			go func() {
				if !test.prompt {
					statuses <- rt.RunScript(ctx, line)
					return
				}
				script, err := rt.ParseSessionInput(line)
				if err != nil {
					t.Errorf("parse %q: %v", line, err)
				}
				statuses <- rt.RunInteractive(ctx, script).Status
			}()
			awaitStalled(t, opened)

			// When
			interrupt()

			// Then
			select {
			case status := <-statuses:
				if status != 130 {
					t.Fatalf("status = %d, want 130", status)
				}
			case <-time.After(time.Second):
				t.Fatalf("%q still runs a second after the interrupt", line)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want nothing said of the ended read", stderr.String())
			}
		})
	}
}

// The INT trap runs after the commands the interrupt ended, and is not cancelled with them: a
// read in it waits for its answer, which comes ten of the pin's intervals after the interrupt.
func TestRuntime_interruptTrapAfterAWaitingCallReadsItsAnswer(t *testing.T) {
	// Given
	path, opened := stall.Pipe(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close(); reader.Close() })
	stdout := &lockedBuffer{}
	rt := runtime.New(applets.NewRegistry(stallApplet()), runtime.Streams{Stdin: reader, Stdout: stdout, Stderr: io.Discard})
	ctx, interrupt := runtime.InterruptContext(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.RunScript(ctx, `trap 'read answer; echo "answered $answer"' INT; stall '`+path+"'\n")
	}()
	awaitStalled(t, opened)

	// When
	interrupt()
	time.Sleep(100 * time.Millisecond)
	fmt.Fprintln(writer, "yes")

	// Then
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("the script still runs; stdout = %q", stdout.String())
	}
	if got := stdout.String(); got != "answered yes\n" {
		t.Fatalf("stdout = %q, want the trap to have read its answer", got)
	}
}

// lockedBuffer is a buffer the test can read while the shell writes it.
type lockedBuffer struct {
	mu   sync.Mutex
	text bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}
