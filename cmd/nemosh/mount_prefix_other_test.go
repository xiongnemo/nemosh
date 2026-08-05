//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestDirectApplet_rejectsRelativeMountPrefixBeforeEffectsForBothEntryForms(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "touch", "mnt/c/effect"}, {"touch", "mnt/c/effect"}} {
		t.Run(args[0], func(t *testing.T) {
			hostCwd := t.TempDir()
			backing := filepath.Join(hostCwd, "mnt", "c")
			if err := os.MkdirAll(backing, 0o700); err != nil {
				t.Fatalf("create host mount fixture: %v", err)
			}
			t.Chdir(hostCwd)
			settings := shellruntime.DefaultPathSettings()
			settings.Config.MountPrefix = "mnt"
			state := shellruntime.State{
				Cwd:   shellruntime.WorkingDirectory(t.TempDir()),
				Paths: &settings,
			}

			stdout, stderr, err := runDirectAppletStateTest(args, state)

			if !errors.Is(err, shellruntime.ErrStateMountPrefixNotAbsolute) {
				t.Fatalf("run(%v): got error %v, want %v", args, err, shellruntime.ErrStateMountPrefixNotAbsolute)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("run(%v): stdout=%q stderr=%q", args, stdout, stderr)
			}
			if _, statErr := os.Stat(filepath.Join(backing, "effect")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("run(%v): effect path error=%v, want not exist", args, statErr)
			}
		})
	}
}
