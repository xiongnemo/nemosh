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
)

type liveOutput struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	updated chan struct{}
}

func newLiveOutput() *liveOutput {
	return &liveOutput{updated: make(chan struct{}, 1)}
}

func (o *liveOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	count, err := o.buffer.Write(data)
	o.mu.Unlock()
	select {
	case o.updated <- struct{}{}:
	default:
	}
	return count, err
}

func (o *liveOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.String()
}

func awaitTextCount(t *testing.T, output *liveOutput, text string, count int) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for strings.Count(output.String(), text) < count {
		select {
		case <-output.updated:
		case <-timer.C:
			t.Fatalf("output = %q, want %d occurrences of %q", output.String(), count, text)
		}
	}
}

func TestRunInteractive_externalNoReadRepromptsWithoutAdditionalInput(t *testing.T) {
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
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
	t.Setenv("NEMOSH_INTERACTIVE_EXTERNAL_HELPER", "1")
	stdout := newLiveOutput()
	stderr := newLiveOutput()
	done := make(chan error, 1)
	cmd := command{stdin: reader, stdout: stdout, stderr: stderr}
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-i"}) }()

	// When
	if _, err := fmt.Fprintln(writer, "PS1='P> '"); err != nil {
		t.Fatalf("write PS1: %v", err)
	}
	awaitTextCount(t, stderr, "P> ", 1)
	commandLine := fmt.Sprintf("'%s' -test.run=^TestInteractiveExternalNoReadHelper$\n", filepath.ToSlash(executable))
	if _, err := io.WriteString(writer, commandLine); err != nil {
		t.Fatalf("write external command: %v", err)
	}

	// Then
	awaitTextCount(t, stderr, "P> ", 2)
	if _, err := fmt.Fprintln(writer, "exit 0"); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	closeInput()
	if err := awaitError(t, done, "external command recovery"); err != nil {
		t.Fatalf("interactive run: %v", err)
	}
	if got := stdout.String(); got != "external-ok\n" {
		t.Fatalf("stdout = %q, want %q", got, "external-ok\n")
	}
}

func TestInteractiveExternalNoReadHelper(t *testing.T) {
	if os.Getenv("NEMOSH_INTERACTIVE_EXTERNAL_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "external-ok")
	os.Exit(0)
}
