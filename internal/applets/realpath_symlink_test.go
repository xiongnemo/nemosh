package applets_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_resolvesDotDotAfterSymlinkTraversal_whenRealpathRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	linkParent := filepath.Join(dir, "links")
	targetParent := filepath.Join(dir, "targets")
	targetDir := filepath.Join(targetParent, "actual")
	mkdirRealpathFixture(t, linkParent)
	mkdirRealpathFixture(t, targetDir)
	linkName := filepath.Join(linkParent, "linked-dir")
	createRealpathSymlink(t, targetDir, linkName)
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{linkName + string(os.PathSeparator) + ".."}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to resolve symlink before dot-dot, got %v", err)
	}
	if got, want := stdout.String(), slashAbs(t, targetParent)+"\n"; got != want {
		t.Fatalf("expected symlink traversal stdout %q, got %q", want, got)
	}
}

func TestDefaultRegistry_printsDanglingSymlinkTarget_whenRealpathRunsOnSymlinkToMissingLeaf(t *testing.T) {
	// Given
	dir := t.TempDir()
	targetParent := filepath.Join(dir, "target")
	mkdirRealpathFixture(t, targetParent)
	target := filepath.Join(targetParent, "missing.txt")
	linkName := filepath.Join(dir, "dangling-link.txt")
	createRealpathSymlink(t, target, linkName)
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{linkName}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to print dangling symlink target with existing parent, got %v", err)
	}
	if got, want := stdout.String(), slashAbs(t, target)+"\n"; got != want {
		t.Fatalf("expected dangling symlink target stdout %q, got %q", want, got)
	}
}

func TestDefaultRegistry_returnsErrExitFalse_whenRealpathRunsOnSymlinkToMissingParent(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := filepath.Join(dir, "missing-parent", "file.txt")
	linkName := filepath.Join(dir, "broken-link.txt")
	createRealpathSymlink(t, target, linkName)
	applet := lookupRealpath(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{linkName}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected missing symlink target parent to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	assertRealpathDiagnostic(t, stderr.String(), linkName)
}

func mkdirRealpathFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("expected fixture directory creation to succeed, got %v", err)
	}
}

func createRealpathSymlink(t *testing.T, target string, linkName string) {
	t.Helper()
	if err := os.Symlink(target, linkName); err != nil {
		message := strings.ToLower(err.Error())
		if os.IsPermission(err) || strings.Contains(message, "privilege") {
			t.Skipf("skipping symlink assertion because this environment lacks symlink permission: %v", err)
		}
		t.Fatalf("expected symlink fixture creation to succeed, got %v", err)
	}
}
