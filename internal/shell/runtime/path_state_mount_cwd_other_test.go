//go:build !windows

package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_cdPreservesMountAliasNativeCwdForRelativeEffects_onNonWindows(t *testing.T) {
	// Given
	leaf := "nemosh-mount-cwd-target"
	mountPrefix := filepath.ToSlash(filepath.Join(t.TempDir(), "mount"))
	aliasDir := filepath.Join(filepath.FromSlash(mountPrefix), "q", leaf)
	command := "nemosh-mount-cwd-probe"
	executable := filepath.Join(aliasDir, executableName(command))
	writeMountCwdFixture(t, aliasDir, executable)

	settings := shellruntime.DefaultPathSettings()
	settings.Config.EnableTmp = false
	settings.Config.MountPrefix = mountPrefix
	var stdout, stderr bytes.Buffer
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout, Stderr: &stderr}, shellruntime.State{
		Cwd:   shellruntime.WorkingDirectory(t.TempDir()),
		Env:   shellruntime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1", "NEMOSH_CHILD_VALUE=alias-child"}),
		Paths: &settings,
	})
	if err != nil {
		t.Fatalf("construct runtime: %v", err)
	}
	aliasPath := mountPrefix + "/q/" + leaf

	// When
	status := rt.RunScript(context.Background(), strings.Join([]string{
		"cd " + shellSingleQuote(aliasPath),
		"pwd",
		"cat applet-input.txt",
		"cat < redirect-input.txt",
		"echo alias-output > redirect-output.txt",
		"touch applet-created.txt",
		"PATH=. " + command + " -test.run=TestRuntimeHelperProcess -- executable",
		"PATH=. " + command + " -test.run=TestRuntimeHelperProcess -- state",
	}, "\n")+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	// The child reports its cwd through getcwd, which the kernel answers with
	// symlinks already resolved. On macOS the temporary directory lives under
	// /var, which is a symlink to /private/var, so the child says /private/var
	// and the launch path says /var -- both correct, describing different
	// things. Resolving the expectation the same way the kernel does keeps the
	// assertion about what this test is for (the alias cwd reaching the child)
	// rather than about the host's symlink layout. A no-op on Linux and Windows.
	wantOutput := strings.Join([]string{
		"/q/" + leaf,
		"alias-applet",
		"alias-redirect",
		executable,
		resolveSymlinks(t, aliasDir),
		"alias-child",
	}, "\n") + "\n"
	if got := stdout.String(); got != wantOutput {
		t.Fatalf("stdout: got %q, want %q", got, wantOutput)
	}
	resolved, err := rt.ResolveNemoshPath("applet-input.txt")
	if err != nil {
		t.Fatalf("resolve relative input: %v", err)
	}
	assertMountCwdResolvedPath(t, resolved, "/q/"+leaf+"/applet-input.txt", filepath.Join(aliasDir, "applet-input.txt"))
	assertFileText(t, filepath.Join(aliasDir, "redirect-output.txt"), "alias-output\n")
	assertFileText(t, filepath.Join(aliasDir, "applet-created.txt"), "")
}

func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %q: %v", path, err)
	}
	return resolved
}

func writeMountCwdFixture(t *testing.T, aliasDir, executable string) {
	t.Helper()
	if err := os.MkdirAll(aliasDir, 0o700); err != nil {
		t.Fatalf("create alias directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aliasDir, "applet-input.txt"), []byte("alias-applet\n"), 0o600); err != nil {
		t.Fatalf("write applet input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aliasDir, "redirect-input.txt"), []byte("alias-redirect\n"), 0o600); err != nil {
		t.Fatalf("write redirect input: %v", err)
	}
	copyRuntimeHelper(t, executable)
}

func assertMountCwdResolvedPath(t *testing.T, got pathmodel.ResolvedPath, canonical, native string) {
	t.Helper()
	if got.Device || got.Canonical != pathmodel.Path(canonical) || !sameNativePath(got.Native, native) {
		t.Fatalf("resolved path: got %+v, want canonical=%q native=%q device=false", got, canonical, native)
	}
}
