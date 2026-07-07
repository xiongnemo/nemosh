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
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected cwd lookup to succeed, got %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("expected cwd change to succeed, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	applet, ok := applets.DefaultRegistry.Lookup("find")
	if !ok {
		t.Fatal("expected find applet to be registered")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err = applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &stderr)

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
