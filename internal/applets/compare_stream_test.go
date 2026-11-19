package applets

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// comm and paste write as they read, as busybox's do: each read all of every operand first, so
// nothing came out until the inputs ended, and `paste - -` of 19 MB peaked at 95 MiB, comm of two
// 4 MB files at 52, where busybox's peaked at 1.1.
func TestCommAndPaste_writeAsTheyRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "right"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	for _, test := range []struct {
		name        string
		applet      Applet
		args        []string
		feed, first string
	}{
		{"paste - -", newPasteApplet(), []string{"-", "-"}, "1\n2\n3\n", "1\t2\n"},
		{"comm - right", newCommApplet(), []string{"-", "right"}, "a\n", "a\n"},
	} {
		// Given
		input, feed := io.Pipe()
		results, output := io.Pipe()
		done := make(chan error, 1)
		go func() {
			err := test.applet.Run(ctx, test.args, input, output, io.Discard)
			output.CloseWithError(err)
			done <- err
		}()
		lines := bufio.NewReader(results)
		written := make(chan string, 1)
		go func() {
			line, _ := lines.ReadString('\n')
			written <- line
		}()

		// When: the input goes on
		if _, err := io.WriteString(feed, test.feed); err != nil {
			t.Fatal(err)
		}

		// Then
		select {
		case line := <-written:
			if line != test.first {
				t.Errorf("%s wrote %q first, want %q", test.name, line, test.first)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("%s wrote nothing while its input went on", test.name)
		}
		feed.Close()
		io.Copy(io.Discard, lines)
		if err := <-done; err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}
