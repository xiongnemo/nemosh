package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_listsDirectoryEntries_whenLsRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("expected ls applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{dir}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected ls to succeed, got %v", err)
	}
	if got := stdout.String(); got != "a.txt\nb.txt\n" {
		t.Fatalf("expected ls output %q, got %q", "a.txt\nb.txt\n", got)
	}
}

func TestDefaultRegistry_copiesFile_whenCpRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	dest := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(source, []byte("copy-me"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("cp")
	if !ok {
		t.Fatal("expected cp applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), []string{source, dest}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cp to succeed, got %v", err)
	}
	contents, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("expected copied destination, got %v", readErr)
	}
	if string(contents) != "copy-me" {
		t.Fatalf("expected copied contents %q, got %q", "copy-me", string(contents))
	}
}

func TestDefaultRegistry_movesFile_whenMvRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	dest := filepath.Join(dir, "dest.txt")
	if err := os.WriteFile(source, []byte("move-me"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("mv")
	if !ok {
		t.Fatal("expected mv applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), []string{source, dest}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected mv to succeed, got %v", err)
	}
	contents, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("expected moved destination, got %v", readErr)
	}
	if string(contents) != "move-me" {
		t.Fatalf("expected moved contents %q, got %q", "move-me", string(contents))
	}
	if _, statErr := os.Stat(source); !os.IsNotExist(statErr) {
		t.Fatalf("expected source to be removed, got stat error %v", statErr)
	}
}

func TestDefaultRegistry_listsFileName_whenLsRunsWithFile(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("expected ls applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected ls to succeed, got %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != path {
		t.Fatalf("expected ls file output %q, got %q", path, got)
	}
}
