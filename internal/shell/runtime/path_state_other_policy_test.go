//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestPathStateOther_resolvesRelativeTraversalFromCanonicalTmp_consistently(t *testing.T) {
	tests := []struct {
		cwd       pathmodel.Path
		input     string
		canonical string
		native    string
	}{
		{cwd: "/tmp", input: "..", canonical: "/", native: "/"},
		{cwd: "/tmp", input: "../child", canonical: "/child", native: "/child"},
		{cwd: "/tmp", input: "../../../child", canonical: "/child", native: "/child"},
		{cwd: "/tmp/subdir", input: "../..", canonical: "/", native: "/"},
		{cwd: "/tmp/subdir", input: "../../child", canonical: "/child", native: "/child"},
		{cwd: "/tmp/subdir", input: "../../../../child", canonical: "/child", native: "/child"},
	}
	for _, tt := range tests {
		t.Run(string(tt.cwd)+"_"+tt.input, func(t *testing.T) {
			// Given
			rt := newOtherPathRuntime(mustOtherWorkingDirectory(t), t.TempDir())
			rt.paths.setWorkingDirectory(pathmodel.ResolvedPath{Canonical: tt.cwd, Native: string(tt.cwd)})

			// When
			resolved, err := rt.ResolveNemoshPath(tt.input)

			// Then
			if err != nil {
				t.Fatalf("resolve %q from %q: %v", tt.input, tt.cwd, err)
			}
			assertOtherResolvedPath(t, resolved, tt.canonical, tt.native)
		})
	}
}

// The fixture is /etc/hosts, not /etc/hostname. hostname is a Linux convention
// and macOS has no such file, so the test failed there on a missing fixture
// rather than on anything it was written to check. hosts is in POSIX-adjacent
// use everywhere this builds and is world-readable on all of them.
func TestPathStateOther_relativeIOAfterTmpTraversal_usesCanonicalNativeDestination(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	backingParent := filepath.Dir(tmpRoot)
	if err := os.MkdirAll(filepath.Join(backingParent, "etc"), 0o700); err != nil {
		t.Fatalf("create conflicting backing parent: %v", err)
	}
	conflict := filepath.Join(backingParent, "etc", "hosts")
	if err := os.WriteFile(conflict, []byte("wrong-backing-parent\n"), 0o600); err != nil {
		t.Fatalf("write conflicting backing file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(conflict) })
	want, err := os.ReadFile("/etc/hosts")
	if err != nil {
		t.Fatalf("read canonical host fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rt := newOtherPathRuntimeWithStreams(mustOtherWorkingDirectory(t), tmpRoot, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cd /tmp\ncat ../etc/hosts\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got := stdout.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("expected canonical host contents %q, got %q", want, got)
	}
}

func TestPathStateOther_appliesConfiguredAliasPolicy_beforeNativeEffects(t *testing.T) {
	// Given
	settings := DefaultPathSettings()
	rt := newOtherPathRuntimeWithSettings(mustOtherWorkingDirectory(t), settings)

	// When
	_, cygdriveErr := rt.ResolveNemoshPath("/cygdrive/c/example.txt")
	mounted, mountErr := rt.ResolveNemoshPath("/mnt/c/example.txt")

	// Then
	if !errors.Is(cygdriveErr, pathmodel.ErrCygdriveDisabled) {
		t.Fatalf("expected ErrCygdriveDisabled, got %v", cygdriveErr)
	}
	if mountErr != nil {
		t.Fatalf("resolve mount alias: %v", mountErr)
	}
	assertOtherResolvedPath(t, mounted, "/c/example.txt", "/mnt/c/example.txt")
}

func TestPathStateOther_acceptsCygdriveAlias_whenConfigured(t *testing.T) {
	// Given
	settings := DefaultPathSettings()
	settings.Config.AcceptCygdrive = true
	rt := newOtherPathRuntimeWithSettings(mustOtherWorkingDirectory(t), settings)

	// When
	resolved, err := rt.ResolveNemoshPath("/cygdrive/c/example.txt")

	// Then
	if err != nil {
		t.Fatalf("resolve enabled cygdrive alias: %v", err)
	}
	assertOtherResolvedPath(t, resolved, "/c/example.txt", "/cygdrive/c/example.txt")
}

func newOtherPathRuntimeWithSettings(cwd string, settings PathSettings) Runtime {
	return NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(cwd),
		Env:   NewEnvironment(nil),
		Paths: &settings,
	})
}
