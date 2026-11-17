package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// migrateIn is a migration of a home of its own, with no other shell running, its output kept.
func migrateIn(t *testing.T, env map[string]string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	run := migrateRun{lookup: lookupFrom(env), stdout: &stdout, stderr: &stderr, others: func() ([]int, error) { return nil, nil }, rename: os.Rename}
	err := run.run(args)
	return stdout.String(), stderr.String(), err
}

func readOrEmpty(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

// `nemosh --migrate` moves what a release up to 1.5 kept to where later ones keep it; with
// --dry-run it says what it would do and touches nothing. Each test is the spec the author's
// tools share: a plain move, the same file on both sides, a different one, and a second run.
func TestMigrate_movesHistoryToTheStateDirectory(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"HOME": home}
	legacy := filepath.Join(home, ".nemosh_history")
	current := filepath.Join(home, ".local", "state", "nemosh", "history")
	writeFile(t, legacy, "echo old\n")

	// A dry run touches nothing.
	stdout, _, err := migrateIn(t, env, "--dry-run")
	if err != nil || !strings.Contains(stdout, "would move") || readOrEmpty(legacy) != "echo old\n" || readOrEmpty(current) != "" {
		t.Fatalf("--dry-run: %q, %v; the old file %q, the new %q", stdout, err, readOrEmpty(legacy), readOrEmpty(current))
	}

	// The move.
	stdout, _, err = migrateIn(t, env)
	if err != nil || !strings.Contains(stdout, "moved") || readOrEmpty(current) != "echo old\n" {
		t.Fatalf("migrate: %q, %v; the new file holds %q", stdout, err, readOrEmpty(current))
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("the old file is still there")
	}

	// And again, which has nothing to do.
	stdout, _, err = migrateIn(t, env)
	if err != nil || !strings.Contains(stdout, "nothing to migrate") {
		t.Fatalf("a second run: %q, %v", stdout, err)
	}
}

func TestMigrate_neverOverwrites(t *testing.T) {
	for _, test := range []struct {
		name, legacy, current string
		conflict              bool
	}{
		{name: "the same on both sides, so the old copy goes", legacy: "same\n", current: "same\n"},
		{name: "different, so both are kept", legacy: "old\n", current: "new\n", conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			legacy := filepath.Join(home, ".nemosh_history")
			current := filepath.Join(home, ".local", "state", "nemosh", "history")
			writeFile(t, legacy, test.legacy)
			writeFile(t, current, test.current)

			// When
			stdout, stderr, err := migrateIn(t, map[string]string{"HOME": home})

			// Then
			if readOrEmpty(current) != test.current {
				t.Fatalf("the new file holds %q, want %q untouched", readOrEmpty(current), test.current)
			}
			_, kept := os.Stat(legacy)
			var status exitStatus
			if test.conflict {
				if kept != nil || !errors.As(err, &status) || status != 1 || !strings.Contains(stdout+stderr, "conflict") {
					t.Fatalf("a conflict: %q, %q, %v, the old file kept: %v", stdout, stderr, err, kept == nil)
				}
				return
			}
			if kept == nil || err != nil {
				t.Fatalf("the same content: %q, %v, the old file kept: %v", stdout, err, kept == nil)
			}
		})
	}
}

// XDG_STATE_HOME, when absolute, is where history goes, as a session would look for it.
func TestMigrate_followsXDGStateHome(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".nemosh_history"), "echo old\n")

	if _, _, err := migrateIn(t, map[string]string{"HOME": home, "XDG_STATE_HOME": state}); err != nil {
		t.Fatal(err)
	}

	if got := readOrEmpty(filepath.Join(state, "nemosh", "history")); got != "echo old\n" {
		t.Fatalf("XDG_STATE_HOME/nemosh/history holds %q", got)
	}
}

// Another shell that is running would go on writing where the history was, so the migration is
// refused while one is, and says which; the shell it was started from does not count.
func TestMigrate_refusesWhileAnotherShellRuns(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".nemosh_history")
	writeFile(t, legacy, "echo old\n")
	var stdout, stderr bytes.Buffer
	run := migrateRun{lookup: lookupFrom(map[string]string{"HOME": home}), stdout: &stdout, stderr: &stderr,
		others: func() ([]int, error) { return []int{4242, 4343}, nil }, rename: os.Rename}

	err := run.run(nil)

	var status exitStatus
	if !errors.As(err, &status) || status != 1 || !strings.Contains(stderr.String(), "4242, 4343") || readOrEmpty(legacy) != "echo old\n" {
		t.Fatalf("with others running: %q, %q, %v; the old file %q", stdout.String(), stderr.String(), err, readOrEmpty(legacy))
	}
}

// Across volumes a rename cannot move a file, so it is copied, checked byte for byte, and only
// then removed, keeping its time.
func TestMigrate_copiesAcrossVolumes(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".nemosh_history")
	writeFile(t, legacy, "echo old\n")
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(legacy, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	run := migrateRun{lookup: lookupFrom(map[string]string{"HOME": home}), stdout: &stdout, stderr: &stderr,
		others: func() ([]int, error) { return nil, nil },
		rename: func(string, string) error { return &os.LinkError{Op: "rename", Err: windows.ERROR_NOT_SAME_DEVICE} }}

	if err := run.run(nil); err != nil {
		t.Fatalf("%v; %q %q", err, stdout.String(), stderr.String())
	}

	current := filepath.Join(home, ".local", "state", "nemosh", "history")
	info, err := os.Stat(current)
	if err != nil || readOrEmpty(current) != "echo old\n" || !info.ModTime().Equal(stamp) {
		t.Fatalf("the copy: %v, %q, %v", err, readOrEmpty(current), info)
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("the old file is still there")
	}
}

// Anything but --dry-run is refused, before anything is looked at -- and so `nemosh --migrate` is
// the option, reached ahead of the shell's own, which would call it invalid.
func TestMigrate_takesOnlyDryRun(t *testing.T) {
	_, stderr, err := migrateIn(t, map[string]string{"HOME": t.TempDir()}, "--force")
	var status exitStatus
	if !errors.As(err, &status) || status != 2 || !strings.Contains(stderr, "--dry-run") {
		t.Fatalf("--force: %q, %v", stderr, err)
	}
	var stdout, viaRun bytes.Buffer
	cmd := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &viaRun}
	err = cmd.run(t.Context(), []string{"nemosh", "--migrate", "--force"})
	if !errors.As(err, &status) || status != 2 || !strings.Contains(viaRun.String(), "takes only --dry-run") {
		t.Fatalf("nemosh --migrate --force: %q, %v", viaRun.String(), err)
	}
}
