package applets

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"
)

// uniq writes a run as soon as the next one begins, as busybox's does, so `tail -f log | uniq`
// shows the lines as they come, and its memory is a line's rather than the input's: it read all
// of its input first, and `uniq -d` of 19 MB peaked at 67 MiB where busybox's peaked at 1.1.
func TestUniq_writesEachRunAsTheNextBegins(t *testing.T) {
	// Given
	input, feed := io.Pipe()
	results, output := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := newUniqApplet().Run(context.Background(), []string{"-c"}, input, output, io.Discard)
		output.CloseWithError(err)
		done <- err
	}()
	lines := bufio.NewReader(results)
	written := make(chan string, 1)
	go func() {
		line, _ := lines.ReadString('\n')
		written <- line
	}()

	// When: a run of two, the first line of the next, and no end of input yet
	if _, err := io.WriteString(feed, "a\na\nb\n"); err != nil {
		t.Fatal(err)
	}

	// Then
	select {
	case line := <-written:
		if line != "      2 a\n" {
			t.Fatalf("the first run was written as %q, want %q", line, "      2 a\n")
		}
	case <-time.After(2 * time.Second):
		feed.Close()
		t.Fatal("nothing was written of the first run while the input went on")
	}
	feed.Close()
	rest, _ := io.ReadAll(lines)
	if err := <-done; err != nil || string(rest) != "      1 b\n" {
		t.Fatalf("the rest was %q, %v; want the run of b", rest, err)
	}
}
