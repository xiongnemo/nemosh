package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Ctrl-C that ends a script ends the jobs it started, as busybox-w32 ends them: measured by
// pressing Ctrl-C in its console, with the job waited for and with it running beside a
// foreground command. The jobs here are goroutines, since a test of this package does not
// allow job processes, and that is the form in which leaving them would show: the job
// would carry on inside the test process and write its marker.
func TestCommand_interruptEndsTheJobsOfTheScript(t *testing.T) {
	marker := filepath.ToSlash(filepath.Join(t.TempDir(), "leaked"))
	signals := make(chan os.Signal, 1)
	stdout := &readyWriter{ready: make(chan struct{})}
	cmd := command{stdin: bytes.NewReader(nil), stdout: stdout, stderr: io.Discard, interrupts: signals}
	script := "{ sleep 1; echo leaked > '" + marker + "'; } &\necho READY\nwait\n"
	done := make(chan error, 1)
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-c", script}) }()
	awaitSignal(t, stdout.ready, "script readiness")

	signals <- os.Interrupt
	err := awaitError(t, done, "interrupted script")

	if status := interactiveStatus(t, err); status != 130 {
		t.Fatalf("status = %d, want 130", status)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the job ran on after Ctrl-C ended the script")
	}
}
