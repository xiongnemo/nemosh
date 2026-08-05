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
	if got := stdout.String(); got != ".\na.txt\nsub\nsub/b.txt\n" {
		t.Fatalf("expected find output %q, got %q", ".\na.txt\nsub\nsub/b.txt\n", got)
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
