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

func TestDefaultRegistry_hidesDotEntries_whenLsRunsWithoutAllFlag(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("v"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o600); err != nil {
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
	if got := stdout.String(); got != "visible.txt\n" {
		t.Fatalf("expected ls output %q, got %q", "visible.txt\n", got)
	}
}

func TestDefaultRegistry_listsDotEntries_whenLsRunsWithClusteredAllLongHumanFlags(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("v"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("expected ls applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-alh", dir}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected ls -alh to succeed, got %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, ".hidden") {
		t.Fatalf("expected ls -alh output to include hidden file, got %q", got)
	}
	if !strings.Contains(got, "visible.txt") {
		t.Fatalf("expected ls -alh output to include visible file, got %q", got)
	}
	if strings.Contains(got, "-alh") {
		t.Fatalf("expected ls -alh not to treat flag as path, got %q", got)
	}
}

func TestDefaultRegistry_printsHumanReadableSize_whenLsRunsWithLongHumanFlags(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1536), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("expected ls applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-lh", dir}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected ls -lh to succeed, got %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "1.5K") {
		t.Fatalf("expected human-readable size in output, got %q", got)
	}
	if !strings.Contains(got, "large.bin") {
		t.Fatalf("expected file name in output, got %q", got)
	}
}

func TestDefaultRegistry_returnsError_whenLsRunsWithUnsupportedFlag(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("ls")
	if !ok {
		t.Fatal("expected ls applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), []string{"-z"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err == nil || err.Error() != "unsupported ls option: -z" {
		t.Fatalf("expected unsupported option error, got %v", err)
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
