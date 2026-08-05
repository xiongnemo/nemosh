//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestPathStateOther_resolvesOrdinaryNativePaths_whenOutsideTmp(t *testing.T) {
	// Given
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	rt := newOtherPathRuntime(cwd, t.TempDir())
	absoluteInput := "/var/nemosh-path-state-absolute.txt"

	// When
	relative, relativeErr := rt.ResolveNemoshPath("relative.txt")
	absolute, absoluteErr := rt.ResolveNemoshPath(absoluteInput)

	// Then
	if relativeErr != nil {
		t.Fatalf("resolve relative path: %v", relativeErr)
	}
	assertOtherResolvedPath(t, relative, filepath.ToSlash(filepath.Join(cwd, "relative.txt")), filepath.Join(cwd, "relative.txt"))
	if absoluteErr != nil {
		t.Fatalf("resolve absolute path: %v", absoluteErr)
	}
	assertOtherResolvedPath(t, absolute, filepath.ToSlash(filepath.Clean(absoluteInput)), filepath.Clean(absoluteInput))
}

func TestPathStateOther_resolvesCanonicalTmpToInjectedBacking(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	rt := newOtherPathRuntime(mustOtherWorkingDirectory(t), tmpRoot)

	// When
	resolved, err := rt.ResolveNemoshPath("/tmp/file.txt")

	// Then
	if err != nil {
		t.Fatalf("resolve /tmp file: %v", err)
	}
	assertOtherResolvedPath(t, resolved, "/tmp/file.txt", filepath.Join(tmpRoot, "file.txt"))
}

func TestNewRuntimeWithState_treatsInitialTmpCwdAsExplicitNativePath(t *testing.T) {
	tmpRoot := t.TempDir()
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(tmpRoot)
	runtime, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   "/tmp/subdir",
		Paths: &settings,
	})
	if err != nil {
		t.Fatalf("construct runtime: %v", err)
	}

	native, nativeErr := runtime.nativeWorkingDirectory()
	resolved, resolveErr := runtime.ResolveNemoshPath("child")

	if nativeErr != nil || resolveErr != nil {
		t.Fatalf("resolve initial native cwd: native=%v resolve=%v", nativeErr, resolveErr)
	}
	if native != "/tmp/subdir" {
		t.Fatalf("native cwd: got %q, want %q", native, "/tmp/subdir")
	}
	assertOtherResolvedPath(t, resolved, "/tmp/subdir/child", "/tmp/subdir/child")
}

func TestPathStateOther_reportsCanonicalTmpCwd_whileRelativeIOUsesBacking(t *testing.T) {
	for _, canonical := range []string{"/tmp", "/tmp/subdir"} {
		t.Run(canonical, func(t *testing.T) {
			// Given
			tmpRoot := t.TempDir()
			if canonical != "/tmp" {
				if err := os.Mkdir(filepath.Join(tmpRoot, "subdir"), 0o700); err != nil {
					t.Fatalf("create backing subdirectory: %v", err)
				}
			}
			var stdout, stderr bytes.Buffer
			rt := newOtherPathRuntimeWithStreams(mustOtherWorkingDirectory(t), tmpRoot, Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), "cd "+canonical+"\npwd\necho contents > child.txt\ncat child.txt\n")

			// Then
			if status != 0 {
				t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
			}
			if got, want := stdout.String(), canonical+"\ncontents\n"; got != want {
				t.Fatalf("expected stdout %q, got %q", want, got)
			}
			backingDir := tmpRoot
			if canonical != "/tmp" {
				backingDir = filepath.Join(tmpRoot, "subdir")
			}
			assertPathFileText(t, filepath.Join(backingDir, "child.txt"), "contents\n")
		})
	}
}

func TestPathStateOther_canonicalizesGeneratedTmpPath_onlyForTmpOrigin(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	rt := newOtherPathRuntime(mustOtherWorkingDirectory(t), tmpRoot)
	native := filepath.Join(tmpRoot, "child.txt")

	// When
	tmpCanonical, tmpErr := rt.CanonicalizeNativePath("/tmp/source", native)
	nativeCanonical, nativeErr := rt.CanonicalizeNativePath(pathmodel.Path(filepath.ToSlash(tmpRoot)), native)

	// Then
	if tmpErr != nil {
		t.Fatalf("canonicalize tmp-generated path: %v", tmpErr)
	}
	if tmpCanonical != "/tmp/child.txt" {
		t.Fatalf("expected tmp canonical path %q, got %q", "/tmp/child.txt", tmpCanonical)
	}
	if nativeErr != nil {
		t.Fatalf("canonicalize ordinary generated path: %v", nativeErr)
	}
	if want := pathmodel.Path(filepath.ToSlash(filepath.Clean(native))); nativeCanonical != want {
		t.Fatalf("expected ordinary canonical path %q, got %q", want, nativeCanonical)
	}
}

func TestPathStateOther_snapshotPreservesTmpIdentity_withoutCrossInstanceMutation(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpRoot, "child"), 0o700); err != nil {
		t.Fatalf("create child backing: %v", err)
	}
	parent := newOtherPathRuntime(mustOtherWorkingDirectory(t), tmpRoot)
	parent.paths.setWorkingDirectory(pathmodel.ResolvedPath{Canonical: "/tmp", Native: tmpRoot})
	child, err := parent.snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot runtime: %v", err)
	}

	// When
	child.paths.setWorkingDirectory(pathmodel.ResolvedPath{Canonical: "/tmp/child", Native: filepath.Join(tmpRoot, "child")})
	childResolved, childErr := child.ResolveNemoshPath("file.txt")
	parentResolved, parentErr := parent.ResolveNemoshPath("file.txt")

	// Then
	if childErr != nil || parentErr != nil {
		t.Fatalf("resolve snapshot paths: child=%v parent=%v", childErr, parentErr)
	}
	if child.WorkingDirectory() != "/tmp/child" || parent.WorkingDirectory() != "/tmp" {
		t.Fatalf("expected isolated canonical cwd child=%q parent=%q, got child=%q parent=%q", "/tmp/child", "/tmp", child.WorkingDirectory(), parent.WorkingDirectory())
	}
	assertOtherResolvedPath(t, childResolved, "/tmp/child/file.txt", filepath.Join(tmpRoot, "child", "file.txt"))
	assertOtherResolvedPath(t, parentResolved, "/tmp/file.txt", filepath.Join(tmpRoot, "file.txt"))
}

func TestPathStateOther_normalizesTmpTraversal_beforeSelectingBacking(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	rt := newOtherPathRuntime(mustOtherWorkingDirectory(t), tmpRoot)

	// When
	resolved, err := rt.ResolveNemoshPath("/tmp/../outside.txt")

	// Then
	if err != nil {
		t.Fatalf("resolve traversing tmp path: %v", err)
	}
	assertOtherResolvedPath(t, resolved, "/outside.txt", "/outside.txt")
}

func newOtherPathRuntime(cwd, tmpRoot string) Runtime {
	return newOtherPathRuntimeWithStreams(cwd, tmpRoot, Streams{})
}

func newOtherPathRuntimeWithStreams(cwd, tmpRoot string, streams Streams) Runtime {
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(tmpRoot)
	return NewWithState(applets.DefaultRegistry, streams, State{
		Cwd:   WorkingDirectory(cwd),
		Env:   NewEnvironment(nil),
		Paths: &settings,
	})
}

func mustOtherWorkingDirectory(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return cwd
}

func assertOtherResolvedPath(t *testing.T, got pathmodel.ResolvedPath, canonical, native string) {
	t.Helper()
	if got.Canonical != pathmodel.Path(canonical) || got.Native != native || got.Device {
		t.Fatalf("expected canonical=%q native=%q device=false, got %+v", canonical, native, got)
	}
}
