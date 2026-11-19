package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/stall"
)

// An applet run directly, `nemosh stall PATH` or under the applet's own name, waiting in a call
// Windows has not answered -- an open of a share that has stopped answering, which took 102 s
// to fail -- is ended by Ctrl-C at once. The applet opens a named pipe nothing is written to, by
// path as a file on a share is opened, and reads it plainly: nothing but the pin on the
// goroutine running it can end that read. The shell's own lines and scripts are the runtime's,
// and its tests hold them (interrupt_waiting_call_test.go there).
type stallApplet struct{}

func (stallApplet) Name() string { return "stall" }

func (stallApplet) Run(_ context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	file, err := os.Open(filepath.FromSlash(args[0]))
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Read(make([]byte, 16))
	return err
}

func TestCommand_interruptEndsADirectAppletWaitingInACall(t *testing.T) {
	for _, name := range []string{directAppletInvocationName("stall"), "nemosh"} {
		// Given
		path, opened := stall.Pipe(t)
		args := []string{name, path}
		if name == "nemosh" {
			args = []string{"nemosh", "stall", path}
		}
		signals := make(chan os.Signal, 1)
		var stderr bytes.Buffer
		cmd := command{stdin: bytes.NewReader(nil), stdout: io.Discard, stderr: &stderr, interrupts: signals,
			registry: applets.NewRegistry(stallApplet{})}
		done := make(chan error, 1)
		go func() { done <- cmd.run(context.Background(), args) }()
		select {
		case <-opened:
		case <-time.After(5 * time.Second):
			t.Fatalf("%v never opened the pipe", args)
		}

		// When
		signals <- os.Interrupt

		// Then
		select {
		case err := <-done:
			if status := interactiveStatus(t, err); status != 130 {
				t.Fatalf("%v: status = %d, want 130", args, status)
			}
		case <-time.After(time.Second):
			t.Fatalf("%v still runs a second after Ctrl-C", args)
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v: stderr = %q, want nothing said of the ended read", args, stderr.String())
		}
	}
}
