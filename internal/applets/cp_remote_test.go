package applets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// A path on another machine is a UNC one or a drive Windows maps to one. Local paths, in either
// spelling, and the device namespace are not.
func TestIsRemotePath(t *testing.T) {
	local := t.TempDir()
	for path, want := range map[string]bool{
		`\\server\share\file`:         true,
		`\\server\share`:              true,
		`\\?\UNC\server\share\file`:   true,
		local:                         false,
		`\\?\` + local:                false,
		`\\.\pipe\nemosh-not-a-share`: false,
		`\\.\NUL`:                     false,
	} {
		if got := isRemotePath(path); got != want {
			t.Errorf("isRemotePath(%q) = %v, want %v", path, got, want)
		}
	}
	saved := driveTypeOf
	t.Cleanup(func() { driveTypeOf = saved })
	driveTypeOf = func(root string) uint32 {
		if strings.EqualFold(root, `Z:\`) {
			return windows.DRIVE_REMOTE
		}
		return windows.DRIVE_FIXED
	}
	if !isRemotePath(`Z:\folder\file`) || !isRemotePath(`\\?\Z:\folder\file`) || isRemotePath(`C:\folder\file`) {
		t.Error("a mapped drive is not told from a local one by its drive type")
	}
}

// A copy to or from a share is cp's own, a megabyte at a time, and never CopyFile2's: a CopyFile2
// that an outage ended left a file of the full size there whose end was zeros, and on a slow link
// Ctrl-C waited for the piece it had under way. mv's copy across volumes still is CopyFile2's,
// which is what keeps a file's streams.
func TestCp_copiesToAndFromAShareItself(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("the bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := pathIsRemote
	t.Cleanup(func() { pathIsRemote = saved })
	for _, remote := range []string{"source", "copy"} {
		pathIsRemote = func(path string) bool { return filepath.Base(path) == remote }
		plain := func(ctx context.Context, source, dest string) error {
			return copyFileAccelerated(ctx, source, dest, false)
		}
		dest := "copy-of-" + remote
		if remote == "copy" {
			dest = "copy"
		}
		stderr, calls, err := runCp(t, dir, plain, "source", dest)
		if err != nil || calls != 0 {
			t.Errorf("cp with %s on a share: %v, %q, %d copies by CopyFile2; want none", remote, err, stderr, calls)
		}
		if data, err := os.ReadFile(filepath.Join(dir, dest)); err != nil || string(data) != "the bytes\n" {
			t.Errorf("the copy with %s on a share holds %q, %v", remote, data, err)
		}
	}
}

// CopyFile2 sizes the file it makes before it copies, so one that fails part-way leaves a file
// that looks complete. That file is removed before the bytes are copied another way; and when it
// cannot be removed, cp says why CopyFile2 failed rather than what the other way met.
func TestCp_aFailedCopyFile2LeavesNoFileOfTheFullSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("the bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failsPartWay := func(_ context.Context, _, dest string) error {
		if err := os.WriteFile(dest, make([]byte, 10), 0o644); err != nil {
			return err
		}
		return windows.ERROR_UNEXP_NET_ERR
	}
	if stderr, _, err := runCp(t, dir, failsPartWay, "source", "copied"); err != nil {
		t.Fatalf("cp: %v, %q", err, stderr)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "copied")); err != nil || string(data) != "the bytes\n" {
		t.Errorf("after CopyFile2 failed the copy holds %q, %v", data, err)
	}

	saved := removeCopyLeftover
	t.Cleanup(func() { removeCopyLeftover = saved })
	removeCopyLeftover = func(string) error { return windows.ERROR_NETNAME_DELETED }
	stderr, calls, err := runCp(t, dir, failsPartWay, "source", "stuck")
	if err == nil || calls != 1 {
		t.Fatalf("cp with the leftover stuck: %v, %q, %d copies by CopyFile2", err, stderr, calls)
	}
	if !strings.Contains(stderr, "'stuck'") || !strings.Contains(stderr, errorText(windows.ERROR_UNEXP_NET_ERR)) {
		t.Errorf("cp said %q; want CopyFile2's own failure named for 'stuck'", stderr)
	}
}

// mv's copy across volumes keeps CopyFile2 whatever the paths, and keeps the source when the
// copy fails.
func TestMv_acrossVolumesToAShareStillKeepsStreams(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("the bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved, savedCopier, savedAt := pathIsRemote, copyAccelerated, cpAccelerateAt
	t.Cleanup(func() { pathIsRemote, copyAccelerated, cpAccelerateAt = saved, savedCopier, savedAt })
	pathIsRemote = func(string) bool { return true }
	calls := 0
	copyAccelerated = func(ctx context.Context, from, to string, keepStreams bool) error {
		calls++
		if !keepStreams {
			t.Error("mv's copy did not keep the streams")
		}
		return windows.ERROR_UNEXP_NET_ERR
	}
	run := &cpRun{ctx: context.Background(), applet: "mv", flags: cpFlags{move: true, recurse: true, preserve: true},
		stderr: &bytes.Buffer{}, stdout: &bytes.Buffer{}}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	run.copyAccelerated(pathOperand{host: source, operand: "source"}, pathOperand{host: filepath.Join(dir, "moved"), operand: "moved"}, info)
	if calls != 1 {
		t.Errorf("mv's copy to a share made %d copies by CopyFile2, want 1", calls)
	}
	if _, err := os.Stat(source); err != nil {
		t.Errorf("the source went: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "moved")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed move left %v", err)
	}
}

func errorText(err windows.Errno) string { return causeText(err) }
