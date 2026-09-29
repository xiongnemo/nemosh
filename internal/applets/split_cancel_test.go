package applets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// split's copy loop stops when the context is cancelled.
//
// Tested against splitInput rather than through the applet, and that is the point of the file
// rather than an implementation detail. The first version of this test ran `split` with an already
// cancelled context, saw context.Canceled come back, and passed -- against code with the check
// removed. It was measuring the reading of INPUT, which is cancellable and ran first. A test that
// cannot fail is worth less than no test, because it also reports that the thing is covered. The
// reader here is a strings.Reader, which knows nothing of the context.
//
// The loop needs a check of its own because nothing else bounds it: reading is cancellable and
// one write is bounded, but `split -l 1` over a large file writes one file per line, and Ctrl-C
// could not stop it.

// cancelView is the smallest ProcessView that resolves paths under a directory.
type cancelView struct{ cwd string }

func (v cancelView) WorkingDirectory() string        { return v.cwd }
func (v cancelView) Environ() []string               { return nil }
func (v cancelView) LookupEnv(string) (string, bool) { return "", false }
func (v cancelView) ResolvePath(path string) string  { return path }

func TestSplitInput_stopsWhenCancelled(t *testing.T) {
	directory := t.TempDir()
	view := cancelView{cwd: directory}
	prefix := filepath.Join(directory, "part_")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When -- one line per file, so an unchecked loop writes five hundred files
	err := splitInput(ctx, view, strings.NewReader(strings.Repeat("line\n", 500)),
		splitSpec{prefix: prefix, suffixLen: 2, count: 1})

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("splitInput returned %v, want a cancellation", err)
	}
	written, globErr := filepath.Glob(prefix + "*")
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(written) != 0 {
		t.Fatalf("wrote %d files under a cancelled context", len(written))
	}
}

// And that it still does the job when it is not cancelled, so the check above cannot pass by
// refusing everything.
func TestSplitInput_writesEveryFile(t *testing.T) {
	directory := t.TempDir()
	prefix := filepath.Join(directory, "part_")

	// When
	err := splitInput(context.Background(), cancelView{cwd: directory}, strings.NewReader("a\nb\nc\nd\n"),
		splitSpec{prefix: prefix, suffixLen: 2, count: 2})

	// Then
	if err != nil {
		t.Fatalf("splitInput: %v", err)
	}
	written, _ := filepath.Glob(prefix + "*")
	if len(written) != 2 {
		t.Fatalf("wrote %d files, want 2", len(written))
	}
	body, err := os.ReadFile(prefix + "aa")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "a\nb\n" {
		t.Fatalf("the first file holds %q", body)
	}
}
