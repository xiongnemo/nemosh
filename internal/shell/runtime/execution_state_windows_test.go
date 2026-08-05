//go:build windows

package runtime

import (
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestNewRuntimeWithState_windowsAcceptsExplicitDriveAliases(t *testing.T) {
	settings := DefaultPathSettings()
	tests := []WorkingDirectory{"/c/work", WorkingDirectory(settings.Config.MountPrefix + "/c/work")}
	for _, cwd := range tests {
		t.Run(string(cwd), func(t *testing.T) {
			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Cwd: cwd, Paths: &settings})

			if err != nil {
				t.Fatalf("construct runtime: %v", err)
			}
		})
	}
}

func TestNewRuntimeWithState_windowsRejectsImplicitCurrentRootCwd(t *testing.T) {
	for _, cwd := range []WorkingDirectory{"/", "/work"} {
		t.Run(string(cwd), func(t *testing.T) {
			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Cwd: cwd})

			if !errors.Is(err, ErrStateCwdNotAbsolute) {
				t.Fatalf("construct runtime: got %v, want %v", err, ErrStateCwdNotAbsolute)
			}
		})
	}
}

func TestNewRuntimeWithState_windowsRejectsVirtualCwdWithoutCurrentRoot(t *testing.T) {
	for _, cwd := range []WorkingDirectory{"/tmp", "/tmp/work", "/dev"} {
		t.Run(string(cwd), func(t *testing.T) {
			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{Cwd: cwd})

			if !errors.Is(err, ErrStateCwdNeedsRoot) {
				t.Fatalf("construct runtime: got %v, want %v", err, ErrStateCwdNeedsRoot)
			}
		})
	}
}

func TestNewRuntimeWithState_windowsRejectsRelativeCwd_whenMountPrefixIsRelative(t *testing.T) {
	settings := DefaultPathSettings()
	settings.Config.MountPrefix = "mnt"

	_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   "mnt/c/work",
		Paths: &settings,
	})

	if !errors.Is(err, ErrStateCwdNotAbsolute) {
		t.Fatalf("construct runtime: got %v, want %v", err, ErrStateCwdNotAbsolute)
	}
}
