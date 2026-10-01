package applets_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// awk waits on standard input only when it reads it. It decoded its standard input before
// running the program, and the decoding peeks for a byte-order mark, so with a pipe that
// stayed open and sent nothing -- a background job's, a tool's -- a BEGIN alone and a list of
// files both waited on it for ever and printed nothing. A program that does read it still
// reads it, decoded.
func TestAwk_doesNotWaitOnStandardInputItDoesNotRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("a 3\nb 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	awk, ok := applets.DefaultRegistry.Lookup("awk")
	if !ok {
		t.Fatal("awk is not registered")
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"BEGIN { print 1 }"}, want: "1\n"},
		{args: []string{"BEGIN { print 1; exit }"}, want: "1\n"},
		{args: []string{"{ print $1 }", "data.txt"}, want: "a\nb\n"},
	} {
		silent, open := io.Pipe()
		var stdout, stderr bytes.Buffer
		done := make(chan error, 1)
		ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
		go func() { done <- awk.Run(ctx, test.args, silent, &stdout, &stderr) }()
		select {
		case err := <-done:
			if err != nil || stdout.String() != test.want {
				t.Errorf("awk %q = %q, %v (%s); want %q", test.args, stdout.String(), err, stderr.String(), test.want)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("awk %q waited on a standard input it does not read", test.args)
		}
		open.Close()
	}
}
