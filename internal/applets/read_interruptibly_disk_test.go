package applets

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// A file on disk is read as it is. A read of one never waits on anyone, so there is nothing for
// a watcher to end, and the goroutine and two channels it took were spent on every read: three
// allocations or more each time, which over a 27 MB file read 4 KiB at a time was 6,500 of them.
// A cancelled context still stops the next read. What may wait -- a pipe, a console -- keeps the
// watcher. A read that hangs on a network share now ends when the share answers or the redirector
// gives up, not at Ctrl-C; busybox-w32's cannot be ended either.
func TestReadInterruptibly_readsADiskFileWithoutAWatcher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, make([]byte, 1<<16), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buffer := make([]byte, 4096)

	// When
	allocations := testing.AllocsPerRun(100, func() {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		if read, err := ReadInterruptibly(ctx, file, buffer); err != nil || read != len(buffer) {
			t.Fatalf("read %d, %v", read, err)
		}
	})

	// Then
	if allocations != 0 {
		t.Fatalf("a read of a disk file allocated %v times; want none", allocations)
	}
	cancel()
	if _, err := ReadInterruptibly(ctx, file, buffer); !errors.Is(err, context.Canceled) {
		t.Fatalf("a read after the cancel gave %v; want the cancellation", err)
	}
}

// Which reads may wait: a pipe's may, a disk file's may not.
func TestReadMayBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	if readMayBlock(file) {
		t.Error("a disk file was taken for one whose read may wait")
	}
	if !readMayBlock(reader) {
		t.Error("a pipe was taken for one whose read cannot wait")
	}
}
