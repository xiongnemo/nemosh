package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runFSApplet(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("expected %s applet to be registered", name)
	}
	var stdout bytes.Buffer
	err := applet.Run(context.Background(), args, &bytes.Buffer{}, &stdout, &bytes.Buffer{})
	return stdout.String(), err
}

func TestMkdir_createsEveryParent_whenGivenTheParentsFlag(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c")

	// When
	_, err := runFSApplet(t, "mkdir", "-p", target)

	// Then
	if err != nil {
		t.Fatalf("mkdir -p: unexpected error %v", err)
	}
	info, statErr := os.Stat(target)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("expected %s to be a directory, stat gave %v", target, statErr)
	}
}

func TestMkdir_doesNotCreateADirectoryNamedAfterTheFlag(t *testing.T) {
	// The flag used to be taken as an operand, so `mkdir -p a/b/c` left a
	// directory literally called -p behind and then failed on the real one.
	// Given
	dir := t.TempDir()

	// When
	_, err := runFSApplet(t, "mkdir", "-p", filepath.Join(dir, "a", "b"))

	// Then
	if err != nil {
		t.Fatalf("mkdir -p: unexpected error %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "-p")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no directory named -p, stat gave %v", statErr)
	}
}

func TestMkdir_staysQuiet_whenTheParentsFlagMeetsAnExistingDirectory(t *testing.T) {
	// Given
	dir := t.TempDir()

	// When
	_, err := runFSApplet(t, "mkdir", "-p", dir)

	// Then
	if err != nil {
		t.Fatalf("mkdir -p on an existing directory: unexpected error %v", err)
	}
}

func TestMkdir_failsOnAnExistingDirectory_whenTheParentsFlagIsAbsent(t *testing.T) {
	// Given
	dir := t.TempDir()

	// When
	_, err := runFSApplet(t, "mkdir", dir)

	// Then
	if err == nil {
		t.Fatal("expected mkdir without -p to refuse an existing directory")
	}
}

func TestMkdir_refusesAnUnknownOption(t *testing.T) {
	// Given
	dir := t.TempDir()

	// When
	_, err := runFSApplet(t, "mkdir", "-Q", filepath.Join(dir, "x"))

	// Then
	if err == nil || !strings.Contains(err.Error(), "Q") {
		t.Fatalf("expected an unknown-option error naming Q, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "x")); !os.IsNotExist(statErr) {
		t.Fatalf("expected nothing to be created, stat gave %v", statErr)
	}
}

func TestRmdir_refusesARegularFile_andLeavesIt(t *testing.T) {
	// os.Remove deleted the file and reported success. rmdir(3) fails with
	// ENOTDIR, and that difference is silent data loss.
	// Given
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// When
	_, err := runFSApplet(t, "rmdir", victim)

	// Then
	if err == nil {
		t.Fatal("expected rmdir to refuse a regular file")
	}
	if _, statErr := os.Stat(victim); statErr != nil {
		t.Fatalf("expected the file to survive, stat gave %v", statErr)
	}
}

func TestRmdir_removesAnEmptyDirectory(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := filepath.Join(dir, "empty")
	if err := os.Mkdir(target, 0o777); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	// When
	_, err := runFSApplet(t, "rmdir", target)

	// Then
	if err != nil {
		t.Fatalf("rmdir: unexpected error %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("expected the directory to be gone, stat gave %v", statErr)
	}
}

func TestRmdir_refusesANonEmptyDirectory(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := filepath.Join(dir, "full")
	if err := os.Mkdir(target, 0o777); err != nil {
		t.Fatalf("seed directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed child: %v", err)
	}

	// When
	_, err := runFSApplet(t, "rmdir", target)

	// Then
	if err == nil {
		t.Fatal("expected rmdir to refuse a non-empty directory")
	}
}

func TestRmdir_removesTheParents_whenGivenTheParentsFlag(t *testing.T) {
	// The walk climbs with dirname and stops at ".", the way busybox's loop
	// does, so it is written relative to make the top of the walk the top of
	// what was asked for.
	// Given
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o777); err != nil {
		t.Fatalf("seed tree: %v", err)
	}
	t.Chdir(dir)

	// When
	_, err := runFSApplet(t, "rmdir", "-p", filepath.Join("a", "b", "c"))

	// Then
	if err != nil {
		t.Fatalf("rmdir -p: unexpected error %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(statErr) {
		t.Fatalf("expected the whole branch to be gone, stat gave %v", statErr)
	}
}
