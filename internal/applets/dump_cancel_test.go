package applets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// split's write loop stops when the context is cancelled.
//
// Tested against writeSplitParts rather than through the applet, and that is the point of the file
// rather than an implementation detail. The first version of this test ran `split` with an already
// cancelled context, saw context.Canceled come back, and passed -- against code with the check
// removed. It was measuring readOperandLines, which is cancellable and runs first. A test that
// cannot fail is worth less than no test, because it also reports that the thing is covered.
//
// The loop needed a check at all because nothing else bounded it: reading is cancellable and one
// write is bounded, but `split -l 1` over a large file writes one file per line, and Ctrl-C could
// not stop it. The context was already being passed to the write, which ignored it.

// cancelView is the smallest ProcessView that resolves paths under a directory.
type cancelView struct{ cwd string }

func (v cancelView) WorkingDirectory() string        { return v.cwd }
func (v cancelView) Environ() []string               { return nil }
func (v cancelView) LookupEnv(string) (string, bool) { return "", false }
func (v cancelView) ResolvePath(path string) string  { return path }

func TestWriteSplitParts_stopsWhenCancelled(t *testing.T) {
	directory := t.TempDir()
	view := cancelView{cwd: directory}
	prefix := filepath.Join(directory, "part_")
	lines := make([]string, 500)
	for index := range lines {
		lines[index] = "line"
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When -- one line per part, so an unchecked loop writes five hundred files
	err := writeSplitParts(ctx, view, prefix, lines, 1)

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("writeSplitParts returned %v, want a cancellation", err)
	}
	written, globErr := filepath.Glob(prefix + "*")
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(written) != 0 {
		t.Fatalf("wrote %d parts under a cancelled context", len(written))
	}
}

// And that it still does the job when it is not cancelled, so the check above cannot pass by
// refusing everything.
func TestWriteSplitParts_writesEveryPart(t *testing.T) {
	directory := t.TempDir()
	prefix := filepath.Join(directory, "part_")

	// When
	err := writeSplitParts(context.Background(), cancelView{cwd: directory}, prefix,
		[]string{"a", "b", "c", "d"}, 2)

	// Then
	if err != nil {
		t.Fatalf("writeSplitParts: %v", err)
	}
	written, _ := filepath.Glob(prefix + "*")
	if len(written) != 2 {
		t.Fatalf("wrote %d parts, want 2", len(written))
	}
	body, err := os.ReadFile(prefix + "aa")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(body)) != "a\nb" {
		t.Fatalf("first part holds %q", body)
	}
}
