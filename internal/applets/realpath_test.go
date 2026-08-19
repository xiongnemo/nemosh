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

func TestDefaultRegistry_registersRealpath_whenLookupByName(t *testing.T) {
	// Given
	name := "realpath"

	// When
	_, ok := applets.DefaultRegistry.Lookup(name)

	// Then
	if !ok {
		t.Fatal("expected realpath applet to be registered")
	}
}

func TestDefaultRegistry_printsAbsolutePath_whenRealpathRunsOnExistingFile(t *testing.T) {
	// Given
	path := writeRealpathFixture(t, t.TempDir(), "file.txt")
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to succeed, got %v", err)
	}
	if got, want := stdout.String(), slashAbs(t, path)+"\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
}

func TestDefaultRegistry_cleansDotDot_whenRealpathRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := writeRealpathFixture(t, filepath.Join(dir, "nested"), "file.txt")
	uncleanPath := filepath.Join(dir, "nested") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "nested" + string(os.PathSeparator) + "file.txt"
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{uncleanPath}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to succeed, got %v", err)
	}
	if got, want := stdout.String(), slashAbs(t, path)+"\n"; got != want {
		t.Fatalf("expected cleaned stdout %q, got %q", want, got)
	}
}

func TestDefaultRegistry_printsEachOperand_whenRealpathRunsWithMultipleFiles(t *testing.T) {
	// Given
	dir := t.TempDir()
	first := writeRealpathFixture(t, dir, "first.txt")
	second := writeRealpathFixture(t, dir, "second.txt")
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{first, second}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to succeed, got %v", err)
	}
	want := slashAbs(t, first) + "\n" + slashAbs(t, second) + "\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected multi-operand stdout %q, got %q", want, got)
	}
}

// This test used to assert the opposite -- that a missing final component was printed and
// exit 0 -- and it was wrong. busybox-w32, GNU and uutils all fail here; printing the path
// is what `realpath -m` does, and neither reference has that option. The behaviour that
// really does print a path that is not there is a *dangling symlink*, which has its own test
// in realpath_symlink_test.go and still passes.
func TestDefaultRegistry_returnsErrExitFalse_whenFinalComponentIsMissing(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "missing.txt")
	applet := lookupRealpath(t)
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected a missing final component to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	// Measured from busybox-w32: `realpath: <operand>: No such file or directory`.
	if want := "No such file or directory\n"; !strings.HasSuffix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to end with %q", stderr.String(), want)
	}
	assertRealpathDiagnostic(t, stderr.String(), path)
}

func TestDefaultRegistry_returnsErrExitFalseAndWritesDiagnostic_whenParentIsMissing(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "missing-parent", "file.txt")
	applet := lookupRealpath(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected missing parent to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	assertRealpathDiagnostic(t, stderr.String(), path)
}

func TestDefaultRegistry_processesAllOperandsAndReturnsErrExitFalse_whenOneRealpathOperandFails(t *testing.T) {
	// Given
	dir := t.TempDir()
	first := writeRealpathFixture(t, dir, "first.txt")
	missing := filepath.Join(dir, "missing-parent", "file.txt")
	second := writeRealpathFixture(t, dir, "second.txt")
	applet := lookupRealpath(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{first, missing, second}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected partial failure to return ErrExitFalse, got %v", err)
	}
	wantStdout := slashAbs(t, first) + "\n" + slashAbs(t, second) + "\n"
	if got := stdout.String(); got != wantStdout {
		t.Fatalf("expected successful operands to be printed %q, got %q", wantStdout, got)
	}
	assertRealpathDiagnostic(t, stderr.String(), missing)
}

func TestDefaultRegistry_resolvesSymlinkTarget_whenRealpathRunsOnSymlink(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := writeRealpathFixture(t, dir, "target.txt")
	linkName := filepath.Join(dir, "linked.txt")
	if err := os.Symlink(target, linkName); err != nil {
		message := strings.ToLower(err.Error())
		if os.IsPermission(err) || strings.Contains(message, "privilege") {
			t.Skipf("skipping symlink assertion because this Windows environment lacks symlink permission: %v", err)
		}
		t.Fatalf("expected symlink fixture creation to succeed, got %v", err)
	}
	applet := lookupRealpath(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{linkName}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected realpath to resolve symlink, got %v", err)
	}
	if got, want := stdout.String(), slashAbs(t, target)+"\n"; got != want {
		t.Fatalf("expected symlink target stdout %q, got %q", want, got)
	}
}

func TestDefaultRegistry_returnsErrExitFalse_whenRealpathRunsWithoutOperands(t *testing.T) {
	// Given
	applet := lookupRealpath(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected no operands to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func writeRealpathFixture(t *testing.T, dir string, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("expected fixture directory creation to succeed, got %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("realpath"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	return path
}

// slashAbs is the canonical spelling realpath is expected to print, which means
// resolving the path the way realpath does rather than only making it absolute.
//
// The difference shows on a machine whose TEMP sits under an 8.3 alias --
// GitHub's Windows runners hand out `C:\Users\RUNNER~1\AppData\Local\Temp`,
// because the profile directory name is longer than eight characters. There
// `t.TempDir()` is the short spelling and realpath answers with the long one,
// so an expectation built from the raw string compares a canonical path against
// a short one and fails for a reason that has nothing to do with realpath.
//
// EvalSymlinks is what does the expansion, and it is the same call realpath
// itself reaches; a path whose leaf does not exist yet cannot be resolved, so
// the parent is resolved instead and the leaf joined back on.
func slashAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("expected absolute path for %q, got %v", path, err)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.ToSlash(resolved)
	}
	parent, leaf := filepath.Split(abs)
	resolvedParent, err := filepath.EvalSymlinks(filepath.Clean(parent))
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(filepath.Join(resolvedParent, leaf))
}

func assertRealpathDiagnostic(t *testing.T, stderr string, path string) {
	t.Helper()
	if !strings.Contains(stderr, "realpath:") {
		t.Fatalf("expected realpath diagnostic prefix, got %q", stderr)
	}
	if !strings.Contains(stderr, filepath.ToSlash(path)) {
		t.Fatalf("expected diagnostic to mention %q, got %q", filepath.ToSlash(path), stderr)
	}
}

func lookupRealpath(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("realpath")
	if !ok {
		t.Fatal("expected realpath applet to be registered")
	}
	return applet
}
