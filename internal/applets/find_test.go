package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_findPrintsTree_whenNoPathProvided(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("expected find fixture write to succeed, got %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatalf("expected find fixture directory to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatalf("expected nested find fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("find")
	if !ok {
		t.Fatal("expected find applet to be registered")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, nil, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected find to succeed, got %v", err)
	}
	// The path operand defaults to `.`, and POSIX writes it verbatim followed by
	// a slash and the rest, so children carry the `./` prefix that busybox,
	// GNU find, and every script that strips it also expect.
	if want := ".\n./a.txt\n./sub\n./sub/b.txt\n"; stdout.String() != want {
		t.Fatalf("expected find output %q, got %q", want, stdout.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

type findTestProcessView struct{ cwd string }

func (v findTestProcessView) WorkingDirectory() string        { return v.cwd }
func (v findTestProcessView) Environ() []string               { return nil }
func (v findTestProcessView) LookupEnv(string) (string, bool) { return "", false }
func (v findTestProcessView) ResolvePath(path string) string  { return filepath.Join(v.cwd, path) }
