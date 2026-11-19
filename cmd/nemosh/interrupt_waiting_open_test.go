package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/testutil/oplock"
)

// A command waiting in a call Windows has not answered -- an open of a share that has stopped
// answering, which took 102 s to fail -- is ended by Ctrl-C at once, as busybox-w32's child
// is. A file whose oplock holder never answers is such an open, on any machine.
func heldFile(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "held.txt")
	if err := os.WriteFile(path, []byte("held\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path), oplock.Hold(t, path)
}

func awaitOplockBreak(t *testing.T, breaking <-chan struct{}) {
	t.Helper()
	select {
	case <-breaking:
	case <-time.After(5 * time.Second):
		t.Fatal("the command never opened the held file")
	}
}

func TestRunInteractive_interruptEndsACommandWaitingInAnOpen(t *testing.T) {
	for _, line := range []string{"cat '%s'", "cat '%s' | cat"} {
		t.Run(line, func(t *testing.T) {
			// Given: the command at the prompt, waiting in its open
			path, breaking := heldFile(t)
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatalf("create stdin pipe: %v", err)
			}
			var closeWriter sync.Once
			closeInput := func() { closeWriter.Do(func() { _ = writer.Close() }) }
			t.Cleanup(func() {
				closeInput()
				_ = reader.Close()
			})
			signals := make(chan os.Signal, 1)
			stdout, stderr := newLiveOutput(), newLiveOutput()
			done := make(chan error, 1)
			cmd := command{stdin: reader, stdout: stdout, stderr: stderr, interrupts: signals}
			go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-i"}) }()
			fmt.Fprintln(writer, "PS1='P> '")
			awaitTextCount(t, stderr, "P> ", 1)
			fmt.Fprintf(writer, line+"\n", path)
			awaitOplockBreak(t, breaking)

			// When
			signals <- os.Interrupt

			// Then
			if !awaitTextCountBounded(stderr, "P> ", 2, time.Second) {
				t.Fatalf("stderr = %q, want the prompt back within a second of Ctrl-C", stderr.String())
			}
			fmt.Fprintln(writer, "echo $?")
			fmt.Fprintln(writer, "exit 0")
			closeInput()
			if err := awaitError(t, done, "interactive session"); err != nil {
				t.Fatalf("interactive run: %v", err)
			}
			if got := stdout.String(); got != "130\n" {
				t.Fatalf("stdout = %q, want the status of an interrupt, 130", got)
			}
			if strings.Contains(stderr.String(), "cat:") {
				t.Fatalf("stderr = %q, want nothing said of the ended open", stderr.String())
			}
		})
	}
}

// The INT trap runs after the commands Ctrl-C ended, and is not cancelled with them: a read in
// it waits for its answer, which comes ten of the pin's intervals after the interrupt.
func TestCommand_interruptTrapAfterAWaitingOpenReadsItsAnswer(t *testing.T) {
	// Given
	path, breaking := heldFile(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})
	signals := make(chan os.Signal, 1)
	stdout := newLiveOutput()
	cmd := command{stdin: reader, stdout: stdout, stderr: io.Discard, interrupts: signals}
	done := make(chan error, 1)
	script := `trap 'read answer; echo "answered $answer"' INT; cat '` + path + `'`
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-c", script}) }()
	awaitOplockBreak(t, breaking)

	// When
	signals <- os.Interrupt
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

func TestCommand_interruptEndsAScriptOrAppletWaitingInAnOpen(t *testing.T) {
	for _, form := range []string{"-c", "applet"} {
		t.Run(form, func(t *testing.T) {
			// Given
			path, breaking := heldFile(t)
			args := []string{"nemosh", "-c", "cat '" + path + "'"}
			if form == "applet" {
				args = []string{"nemosh", "cat", path}
			}
			signals := make(chan os.Signal, 1)
			var stderr bytes.Buffer
			cmd := command{stdin: bytes.NewReader(nil), stdout: io.Discard, stderr: &stderr, interrupts: signals}
			done := make(chan error, 1)
			go func() { done <- cmd.run(context.Background(), args) }()
			awaitOplockBreak(t, breaking)

			// When
			signals <- os.Interrupt

			// Then
			select {
			case err := <-done:
				if status := interactiveStatus(t, err); status != 130 {
					t.Fatalf("status = %d, want 130", status)
				}
			case <-time.After(time.Second):
				t.Fatalf("%v still runs a second after Ctrl-C", args)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want nothing said of the ended open", stderr.String())
			}
		})
	}
}
