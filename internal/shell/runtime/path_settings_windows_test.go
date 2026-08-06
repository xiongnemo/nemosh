//go:build windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The alias and virtual-root knobs in docs/design/windows-path-model.md are
// configurable by design -- "do not collapse /c and /mnt/c into one generic
// alias knob" -- but every test that switches one off runs under `!windows`.
// These four drive the switches on the platform the knobs exist for, where a
// resolved path has to come back out as a real drive spelling.

// Cygdrive is off by default and is not a v0 compatibility goal, so the only
// thing worth pinning is that turning it on produces an ordinary native path
// rather than a second kind of path that leaks into `pwd`.
func TestP05WaveA_WindowsPathPolicy_acceptsConfiguredCygdrivePathsEndToEnd(t *testing.T) {
	// Given
	native := t.TempDir()
	canonical := string(canonicalWindowsPath(t, native))
	settings := DefaultPathSettings()
	settings.Config.AcceptCygdrive = true
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
		Cwd:   WorkingDirectory(native),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(),
		"cd /cygdrive"+canonical+"\npwd\necho reached > reached.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	// The alias is an input spelling only: `pwd` reports the drive form.
	if got, want := stdout.String(), canonical+"\n"; got != want {
		t.Fatalf("expected pwd %q, got %q", want, got)
	}
	assertPathFileText(t, filepath.Join(native, "reached.txt"), "reached\n")
}

// The mount prefix is a configured string, not the literal `/mnt`. Moving it
// has to move the alias, which means the old spelling stops being one.
func TestP05WaveA_WindowsPathPolicy_honoursAConfiguredMountPrefix(t *testing.T) {
	// Given
	native := t.TempDir()
	canonical := string(canonicalWindowsPath(t, native))
	settings := DefaultPathSettings()
	settings.Config.MountPrefix = "/media"
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
		Cwd:   WorkingDirectory(native),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(),
		"cd /media"+canonical+"\npwd\necho reached > reached.txt\n")
	stale, staleErr := rt.ResolveNemoshPath("/mnt/c/example.txt")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got, want := stdout.String(), canonical+"\n"; got != want {
		t.Fatalf("expected pwd %q, got %q", want, got)
	}
	assertPathFileText(t, filepath.Join(native, "reached.txt"), "reached\n")
	if staleErr != nil {
		t.Fatalf("resolve the former mount prefix: %v", staleErr)
	}
	// /mnt is now an ordinary directory name under the current root.
	if want := underCurrentRoot(native, "mnt", "c", "example.txt"); !sameWindowsPath(stale.Native, want) {
		t.Fatalf("expected the former prefix to resolve to %q, got %q", want, stale.Native)
	}
}

// A disabled alias or virtual root does not become an error; it stops being
// special and resolves under the current root like any other absolute path.
func TestP05WaveA_WindowsPathPolicy_resolvesDisabledAliasesUnderTheCurrentRoot(t *testing.T) {
	tests := []struct {
		name    string
		disable func(*PathSettings)
		input   string
		want    []string
	}{
		{
			name:    "mount prefix",
			disable: func(s *PathSettings) { s.Config.EnableMountPath = false },
			input:   "/mnt/c/example.txt",
			want:    []string{"mnt", "c", "example.txt"},
		},
		{
			name:    "tmp",
			disable: func(s *PathSettings) { s.Config.EnableTmp = false },
			input:   "/tmp/example.txt",
			want:    []string{"tmp", "example.txt"},
		},
		{
			name:    "dev",
			disable: func(s *PathSettings) { s.Config.EnableDev = false },
			input:   "/dev/null",
			want:    []string{"dev", "null"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			native := t.TempDir()
			settings := DefaultPathSettings()
			tt.disable(&settings)
			rt := NewWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(native),
				Paths: &settings,
			})

			// When
			resolved, err := rt.ResolveNemoshPath(tt.input)

			// Then
			if err != nil {
				t.Fatalf("resolve %q with %s disabled: %v", tt.input, tt.name, err)
			}
			if resolved.Device {
				t.Fatalf("expected %q not to resolve as a device once %s is disabled", tt.input, tt.name)
			}
			if want := underCurrentRoot(native, tt.want...); !sameWindowsPath(resolved.Native, want) {
				t.Fatalf("expected %q to resolve to %q, got %q", tt.input, want, resolved.Native)
			}
		})
	}
}

// docs/design/windows-path-model.md:24 maps /tmp to %TEMP% or %TMP%. Every other
// tmp test injects a TmpRoot, so nothing checks what the default actually is.
func TestP05WaveA_WindowsPathPolicy_backsVirtualTmpWithTheHostTemporaryDirectory(t *testing.T) {
	// Given
	hostTemp := os.Getenv("TMP")
	if hostTemp == "" {
		hostTemp = os.Getenv("TEMP")
	}
	if hostTemp == "" {
		t.Skip("neither TMP nor TEMP is set in the environment")
	}
	rt := NewWithState(applets.DefaultRegistry, Streams{}, State{Cwd: WorkingDirectory(t.TempDir())})

	// When
	resolved, err := rt.ResolveNemoshPath("/tmp/child.txt")

	// Then
	if err != nil {
		t.Fatalf("resolve the default tmp backing: %v", err)
	}
	if resolved.Canonical != "/tmp/child.txt" {
		t.Fatalf("expected canonical /tmp/child.txt, got %q", resolved.Canonical)
	}
	// Compared by identity rather than spelling: %TMP% is often the 8.3 form.
	wantInfo, err := os.Stat(hostTemp)
	if err != nil {
		t.Fatalf("stat the host temporary directory %q: %v", hostTemp, err)
	}
	gotInfo, err := os.Stat(filepath.Dir(resolved.Native))
	if err != nil {
		t.Fatalf("stat the tmp backing %q: %v", filepath.Dir(resolved.Native), err)
	}
	if !os.SameFile(wantInfo, gotInfo) {
		t.Fatalf("expected /tmp to be backed by %q, got %q", hostTemp, filepath.Dir(resolved.Native))
	}
}

// underCurrentRoot spells a path the way the current root does when a leading
// slash is taken literally: the drive the given directory sits on, then the
// segments as typed.
func underCurrentRoot(inside string, segments ...string) string {
	volume := filepath.VolumeName(inside)
	return filepath.Join(append([]string{volume + string(os.PathSeparator)}, segments...)...)
}
