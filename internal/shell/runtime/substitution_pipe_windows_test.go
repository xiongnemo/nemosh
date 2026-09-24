//go:build windows

package runtime

import (
	"io"
	"os"
	"testing"
)

// A consumer can open the pipe, write and close it before the command's goroutine asks for
// a connection. What it wrote is still there, and must reach the command: taking that for an
// empty input lost `echo hi > >(cat)` on every machine faster than the goroutine.
func TestSubstitutionPipe_deliversWhatWasWrittenBeforeTheReaderAsked(t *testing.T) {
	pipe, err := newSubstitutionPipe()
	if err != nil {
		t.Fatal(err)
	}
	writer, err := os.OpenFile(pipe.path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("early\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := pipe.accept()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "early\n" {
		t.Fatalf("read %q (err %v), want what was written before the reader asked", data, err)
	}
}

// A path nobody opened is let go of by abandon, and the command reads an empty input.
func TestSubstitutionPipe_abandonGivesAnEmptyInput(t *testing.T) {
	pipe, err := newSubstitutionPipe()
	if err != nil {
		t.Fatal(err)
	}
	pipe.abandon()
	reader, err := pipe.accept()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if data, err := io.ReadAll(reader); err != nil || len(data) != 0 {
		t.Fatalf("read %q (err %v), want nothing", data, err)
	}
}
