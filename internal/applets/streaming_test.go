package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// Whether an applet answers **before its input ends**.
//
// Every other test in this package feeds a whole input through a strings.Reader, which has
// already ended by the time the applet reads it -- so none of them can tell a filter that
// streams from one that waits, and none of them noticed that an interactive `bc` printed
// nothing at all. These hold the input open on purpose.
//
// Two faults were found this way and both are fixed here:
//
//   - **The byte-order mark was looked for by waiting for three bytes.** Typing `1` and
//     Enter is two, so `bc`, `dc` and `ed` read the line and were never given it. Every
//     applet that interprets text went through that reader.
//   - **awk and tr held their output** until the input ended. busybox's awk and tr both
//     stream, and `tail -f log | awk '...'` is the reason it matters.
//
// What is deliberately *not* asserted is the opposite: `sed`, `uniq`, `sort`, `wc`, the
// checksums and the dump tools all wait for the end, and so do busybox's. Some of them must
// -- a digest has no partial answer -- and the rest match the reference. Pinning that down
// would turn a later improvement into a failing test.

// answersBeforeInputEnds feeds one line with the input still open and reports what came back.
func answersBeforeInputEnds(t *testing.T, name string, args []string, feed, want string) {
	t.Helper()
	applet, found := DefaultRegistry.Lookup(name)
	if !found {
		t.Fatalf("%s is not registered", name)
	}
	reader, writer := io.Pipe()
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- applet.Run(context.Background(), args, reader, out, io.Discard)
	}()
	defer func() {
		writer.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Errorf("%s did not finish after its input was closed", name)
		}
	}()
	fmt.Fprint(writer, feed)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s %v: nothing containing %q arrived while its input was still open; got %q",
		name, args, want, out.String())
}

// TestFiltersAnswerBeforeTheInputEnds covers the applets that should produce a line when a
// line arrives, which is what makes `tail -f log | ...` work.
func TestFiltersAnswerBeforeTheInputEnds(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name string
		args []string
		feed string
		want string
	}{
		{name: "cat", feed: "hello there\n", want: "hello there\n"},
		{name: "grep", args: []string{"hello"}, feed: "hello there\n", want: "hello there\n"},
		{name: "awk", args: []string{"{print $1}"}, feed: "hello there\n", want: "hello\n"},
		{name: "tr", args: []string{"a-z", "A-Z"}, feed: "hello there\n", want: "HELLO THERE\n"},
		{name: "nl", feed: "hello\n", want: "hello"},
		{name: "cut", args: []string{"-c1"}, feed: "hello\n", want: "h\n"},
		{name: "rev", feed: "abc\n", want: "cba\n"},
		{name: "tee", feed: "hello\n", want: "hello\n"},
		{name: "fold", args: []string{"-w2"}, feed: "abcd\n", want: "ab\n"},
		{name: "expand", feed: "hello\n", want: "hello\n"},
		{name: "ts", args: []string{"%Y"}, feed: "hello\n", want: "hello\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			answersBeforeInputEnds(t, testcase.name, testcase.args, testcase.feed, testcase.want)
		})
	}
}

// TestShortFirstLineIsNotSwallowed is the byte-order-mark fault on its own.
//
// Two bytes is what typing a single character and pressing Enter really sends, and the mark
// detection used to wait for a third. Every case here is under three bytes on purpose.
func TestShortFirstLineIsNotSwallowed(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name string
		args []string
		feed string
		want string
	}{
		{name: "bc", feed: "1\n", want: "1\n"},
		{name: "dc", feed: "5p\n", want: "5\n"},
		{name: "ed", args: []string{"-s"}, feed: "=\n", want: "0\n"},
		{name: "cat", feed: "a\n", want: "a\n"},
		{name: "awk", args: []string{"{print}"}, feed: "a\n", want: "a\n"},
		{name: "grep", args: []string{"a"}, feed: "a\n", want: "a\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			answersBeforeInputEnds(t, testcase.name, testcase.args, testcase.feed, testcase.want)
		})
	}
}

// TestAwkBuffersToAFile is the other half of awk's flushing rule: a regular file is not
// watched, so it is not flushed per record.
//
// Asserted through the decision itself rather than by timing a write, because a test that
// measured syscalls would be measuring the operating system.
func TestAwkBuffersToAFile(t *testing.T) {
	t.Parallel()
	// A pipe, a terminal, and anything the shell has wrapped all count as watched.
	if writerIsRegularFile(&lockedBuffer{}) {
		t.Fatal("a wrapped writer was taken for a regular file, so nothing would ever flush per record")
	}
	file, err := os.CreateTemp(t.TempDir(), "awk-out-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	if !writerIsRegularFile(file) {
		t.Fatal("a file on disk was taken for something watched, so every record would be flushed")
	}
}
