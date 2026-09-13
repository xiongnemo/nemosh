package runtime_test

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **An applet blocked reading the terminal lets go when the command is interrupted.**
//
// The first attempt at this fix went into internal/applets and never ran: inside the shell
// an applet's stdin is not an *os.File but a descriptorReader, whose ReadContext forwards
// here, and only here is the underlying file in hand. The test that covered it used an
// io.Pipe, which is not an *os.File either, so it passed against code that could not work.
// That is the shape of this tree's recurring mistake -- a wrapper chain answered at the
// wrong hop -- and the reason this test uses os.Pipe, whose ends really are files, and goes
// through the runtime rather than calling the helper.
//
// Windows only, because interruptPipeIO is CancelIoEx and is a no-op elsewhere. Away from
// Windows the blocked read stays blocked, exactly as it did before.

func TestRuntime_interruptReleasesAnAppletBlockedOnItsInput(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })

	ctx, interrupt, release := runtime.InterruptContextWithRelease(context.Background())
	defer release()
	out := &firingWriter{fire: func() {}}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: reader, Stdout: out, Stderr: io.Discard})

	if _, err := writer.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}

	script, err := runtime.ParseScript("cat\n")
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan int, 1)
	go func() { finished <- rt.RunInteractive(ctx, script).Status }()

	// The read has to be **already blocked** before the interrupt, which is the whole
	// point: cancelling before one starts was always handled, by the check at the top of
	// readWithContext. The first version of this test interrupted from inside the write and
	// passed against the unfixed code for exactly that reason.
	//
	// Echoing the line is the signal that cat has consumed the input and gone back for
	// more; the pause after it is for the read to reach the syscall. Erring short would
	// make this pass when it should not, so it is generous rather than tight.
	deadline := time.Now().Add(5 * time.Second)
	for out.String() != "hello\n" {
		if time.Now().After(deadline) {
			t.Fatal("cat never echoed the line, so it never reached the read this is about")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	interrupt()

	select {
	case status := <-finished:
		if status != 130 {
			t.Errorf("status = %d, want 130", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cat is still blocked on a read the interrupt should have ended; at a terminal " +
			"this is Ctrl-C doing nothing until the next keystroke, which is then eaten noticing")
	}
	if got := out.String(); got != "hello\n" {
		t.Errorf("stdout = %q, want the line that was written before the interrupt", got)
	}
}
