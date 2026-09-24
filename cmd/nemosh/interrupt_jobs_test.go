package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ctrl-C that ends a script ends the jobs it started, as busybox-w32 ends them: measured by
// pressing Ctrl-C in its console, with the job waited for and with it running beside a
// foreground command. It says so, and names the choice, since bash would have left them
// running. The jobs here are goroutines, since a test of this package does not allow job
// processes, and that is the form in which leaving them would show: the job would carry on
// inside the test process and write its marker.
func TestCommand_interruptEndsTheJobsOfTheScript(t *testing.T) {
	marker := filepath.ToSlash(filepath.Join(t.TempDir(), "leaked"))
	stderr, err := interruptScript(t, "{ sleep 1; echo leaked > '"+marker+"'; } &\necho READY\nwait\n")

	if status := interactiveStatus(t, err); status != 130 {
		t.Fatalf("status = %d, want 130", status)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the job ran on after Ctrl-C ended the script")
	}
	want := "nemosh: Ctrl-C ended the script, and the background job it had running: [1]\n" +
		"hint: busybox ends a script's jobs on Ctrl-C too; bash would have left them running\n"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr %q, want it to say which job went and why", stderr)
	}
}

// With no job left running there is nothing to say, so nothing is said.
func TestCommand_interruptSaysNothingWhenNoJobWasRunning(t *testing.T) {
	stderr, err := interruptScript(t, "true &\nwait\necho READY\nsleep 5\n")

	if status := interactiveStatus(t, err); status != 130 {
		t.Fatalf("status = %d, want 130", status)
	}
	if strings.Contains(stderr, "Ctrl-C ended") || strings.Contains(stderr, "hint:") {
		t.Fatalf("stderr %q, want no notice", stderr)
	}
}

// interruptScript runs script under `nemosh -c`, presses Ctrl-C once it prints READY, and
// answers with its stderr and its result.
func interruptScript(t *testing.T, script string) (string, error) {
	t.Helper()
	signals := make(chan os.Signal, 1)
	stdout := &readyWriter{ready: make(chan struct{})}
	var stderr syncBuffer
	cmd := command{stdin: bytes.NewReader(nil), stdout: stdout, stderr: &stderr, interrupts: signals}
	done := make(chan error, 1)
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-c", script}) }()
	awaitSignal(t, stdout.ready, "script readiness")
	signals <- os.Interrupt
	err := awaitError(t, done, "interrupted script")
	return stderr.String(), err
}

// syncBuffer is a bytes.Buffer that a job's goroutine and the test can share.
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
