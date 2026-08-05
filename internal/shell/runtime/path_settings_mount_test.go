package runtime

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestNewRuntimeWithState_rejectsInvalidEnabledMountPrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr error
	}{
		{name: "relative", prefix: "mnt", wantErr: ErrStateMountPrefixNotAbsolute},
		{name: "trailing slash", prefix: "/mnt/", wantErr: ErrStateMountPrefixNotCanonical},
		{name: "dot segment", prefix: "/mnt/./nested", wantErr: ErrStateMountPrefixNotCanonical},
		{name: "parent segment", prefix: "/mnt/../alias", wantErr: ErrStateMountPrefixNotCanonical},
		{name: "repeated root", prefix: "//mnt", wantErr: ErrStateMountPrefixNotCanonical},
		{name: "root", prefix: "/", wantErr: ErrStateMountPrefixNeedsSegment},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultPathSettings()
			settings.Config.MountPrefix = test.prefix

			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(t.TempDir()),
				Paths: &settings,
			})

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("construct runtime: got %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestNewRuntimeWithState_acceptsInvalidMountPrefix_whenMountPathsDisabled(t *testing.T) {
	for _, prefix := range []string{"mnt", "/mnt/", "/"} {
		t.Run(prefix, func(t *testing.T) {
			settings := DefaultPathSettings()
			settings.Config.EnableMountPath = false
			settings.Config.MountPrefix = prefix

			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(t.TempDir()),
				Paths: &settings,
			})

			if err != nil {
				t.Fatalf("construct runtime with disabled mount paths: %v", err)
			}
		})
	}
}

func TestNewRuntimeWithState_validatesCwdThenTmpRootThenMountPrefix(t *testing.T) {
	settings := DefaultPathSettings()
	settings.TmpRoot = "relative-tmp"
	settings.Config.MountPrefix = "mnt"

	_, cwdErr := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Paths: &settings})
	_, tmpErr := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})
	settings.TmpRoot = WorkingDirectory(t.TempDir())
	_, mountErr := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	if !errors.Is(cwdErr, ErrStateCwdRequired) {
		t.Fatalf("cwd precedence: got %v, want %v", cwdErr, ErrStateCwdRequired)
	}
	if !errors.Is(tmpErr, ErrStateTmpRootNotAbsolute) {
		t.Fatalf("tmp precedence: got %v, want %v", tmpErr, ErrStateTmpRootNotAbsolute)
	}
	if !errors.Is(mountErr, ErrStateMountPrefixNotAbsolute) {
		t.Fatalf("mount validation: got %v, want %v", mountErr, ErrStateMountPrefixNotAbsolute)
	}
}

func TestNewRuntimeWithState_acceptsSafeEnabledMountPrefixes(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		input  string
	}{
		{name: "default", input: "/mnt/c/example"},
		{name: "custom", prefix: "/volumes", input: "/volumes/c/example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultPathSettings()
			settings.Config.MountPrefix = test.prefix
			runtime, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(t.TempDir()),
				Paths: &settings,
			})
			if err != nil {
				t.Fatalf("construct runtime: %v", err)
			}

			resolved, err := runtime.ResolveNemoshPath(test.input)
			if err != nil {
				t.Fatalf("resolve mount path: %v", err)
			}
			if resolved.Canonical != "/c/example" || !filepath.IsAbs(resolved.Native) {
				t.Fatalf("resolved mount path: got %+v, want canonical /c/example with absolute native", resolved)
			}
		})
	}
}
