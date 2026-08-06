//go:build windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// uncTemporaryDirectory returns a temporary directory together with a UNC
// spelling that reaches the same directory, or skips when the machine exposes
// none. Windows always publishes an administrative share per drive, `C$`, but
// only to an elevated caller; a plainly shared drive letter is tried first
// because it keeps `$` out of the scripts below.
func uncTemporaryDirectory(t *testing.T) (native, unc string) {
	t.Helper()
	native = t.TempDir()
	volume := filepath.VolumeName(native)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("temporary directory %q is not on a drive letter", native)
	}
	below := strings.TrimPrefix(native, volume)
	nativeInfo, err := os.Stat(native)
	if err != nil {
		t.Fatalf("stat the temporary directory: %v", err)
	}
	for _, share := range []string{volume[:1], volume[:1] + "$"} {
		candidate := `\\localhost\` + share + below
		info, err := os.Stat(candidate)
		if err != nil || !os.SameFile(nativeInfo, info) {
			continue
		}
		return native, candidate
	}
	t.Skipf("no reachable UNC spelling of %q; publish a share for %s or run elevated", native, volume)
	return "", ""
}

// A UNC share is a current root in its own right, so `/` under it is the share
// and not the drive the share happens to sit on. busybox-w32 ash behaves the
// same way -- `cd //192.168.1.200/Media/navidrome; cd /; pwd` prints
// `//192.168.1.200/Media` -- and docs/design/windows-path-model.md:174 records
// that as the rule Nemosh follows.
func TestP05WaveA_ShellIO_treatsAUNCShareAsTheCurrentRoot(t *testing.T) {
	// Given
	native, unc := uncTemporaryDirectory(t)
	inner := filepath.Join(native, "inner")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatalf("create the inner directory: %v", err)
	}
	canonicalInner := string(canonicalWindowsPath(t, filepath.Join(unc, "inner")))
	canonicalShare := string(canonicalWindowsPath(t, `\\localhost\`+shareOf(t, unc)))
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
		Cwd: WorkingDirectory(t.TempDir()),
	})

	// When
	status := rt.RunScript(context.Background(),
		"cd "+filepath.ToSlash(unc)+"/inner\npwd\necho reached > made.txt\ncd /\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got, want := stdout.String(), canonicalInner+"\n"+canonicalShare+"\n"; got != want {
		t.Fatalf("expected pwd output %q, got %q", want, got)
	}
	// The relative write landed in the directory the UNC path names, which is
	// the same directory the drive path names.
	assertPathFileText(t, filepath.Join(inner, "made.txt"), "reached\n")
}

// An absolute path typed while the current root is a share resolves under that
// share, which is the half of the rule the test above cannot show on its own.
// The share here is usually the drive root, so the file would land in the same
// place either way; what tells the two apart is that `pwd` keeps the UNC
// spelling rather than falling back to the drive.
func TestP05WaveA_ShellIO_resolvesAbsolutePathsUnderTheUNCCurrentRoot(t *testing.T) {
	// Given
	native, unc := uncTemporaryDirectory(t)
	below := strings.TrimPrefix(filepath.ToSlash(unc), "//localhost/"+shareOf(t, unc))
	canonicalUNC := string(canonicalWindowsPath(t, unc))
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
		Cwd: WorkingDirectory(t.TempDir()),
	})

	// When
	status := rt.RunScript(context.Background(),
		"cd "+filepath.ToSlash(unc)+"\ncd "+below+"\npwd\necho absolute > absolute.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got, want := stdout.String(), canonicalUNC+"\n"; got != want {
		t.Fatalf("expected pwd %q, got %q", want, got)
	}
	assertPathFileText(t, filepath.Join(native, "absolute.txt"), "absolute\n")
}

func shareOf(t *testing.T, unc string) string {
	t.Helper()
	fields := strings.Split(filepath.ToSlash(strings.TrimPrefix(unc, `\\`)), "/")
	if len(fields) < 2 {
		t.Fatalf("cannot read a share name out of %q", unc)
	}
	return fields[1]
}
