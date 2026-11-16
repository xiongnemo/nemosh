package applets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// runCp runs cp in dir with every regular file copied by CopyFile2, counting the copies it made
// that way.
func runCp(t *testing.T, dir string, copier func(context.Context, string, string) error, args ...string) (string, int, error) {
	t.Helper()
	savedAt, savedCopier := cpAccelerateAt, copyAccelerated
	t.Cleanup(func() { cpAccelerateAt, copyAccelerated = savedAt, savedCopier })
	calls := 0
	cpAccelerateAt = 0
	copyAccelerated = func(ctx context.Context, source, dest string, keepStreams bool) error {
		calls++
		if keepStreams {
			t.Errorf("cp asked to keep %s's alternate data streams", source)
		}
		return copier(ctx, source, dest)
	}
	applet, _ := DefaultRegistry.Lookup("cp")
	var stderr bytes.Buffer
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: os.Environ()})
	err := applet.Run(ctx, args, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	return stderr.String(), calls, err
}

// A file copied by CopyFile2 is what cp's own copy would be: its bytes, the time it was made, the
// archive bit and not the source's hidden one, and none of its alternate data streams. -p keeps
// the source's time; a file at the destination is replaced.
func TestCp_copiesARegularFileByCopyFile2(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("the bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source+":side", []byte("a stream"), 0o644); err != nil {
		t.Skipf("no alternate data streams here: %v", err)
	}
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(source, past, past); err != nil {
		t.Fatal(err)
	}
	name, _ := windows.UTF16PtrFromString(source)
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "there"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		args     []string
		dest     string
		keepTime bool
		calls    int
	}{
		{args: []string{"source", "plain"}, dest: "plain", calls: 1},
		{args: []string{"-p", "source", "kept"}, dest: "kept", keepTime: true, calls: 1},
		// Refused for the file in the way, and made once it is gone.
		{args: []string{"source", "there"}, dest: "there", calls: 2},
	} {
		plain := func(ctx context.Context, source, dest string) error {
			return copyFileAccelerated(ctx, source, dest, false)
		}
		stderr, calls, err := runCp(t, dir, plain, test.args...)
		if err != nil || calls != test.calls {
			t.Fatalf("cp %q: %v, %q, %d copies by CopyFile2", test.args, err, stderr, calls)
		}
		dest := filepath.Join(dir, test.dest)
		if data, err := os.ReadFile(dest); err != nil || string(data) != "the bytes\n" {
			t.Errorf("%s holds %q, %v", test.dest, data, err)
		}
		if _, err := os.Stat(dest + ":side"); err == nil {
			t.Errorf("%s carried the alternate data stream", test.dest)
		}
		destName, _ := windows.UTF16PtrFromString(dest)
		if attributes, err := windows.GetFileAttributes(destName); err != nil || attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0 {
			t.Errorf("%s has attributes %#x, %v; want the hidden bit gone", test.dest, attributes, err)
		}
		info, err := os.Stat(dest)
		if err != nil {
			t.Fatal(err)
		}
		if kept := info.ModTime().Equal(past); kept != test.keepTime {
			t.Errorf("%s has time %v; kept the source's: %v, want %v", test.dest, info.ModTime(), kept, test.keepTime)
		}
	}
}

// A copy cancelled under way, as Ctrl-C cancels it, fails and leaves no file; one CopyFile2
// refuses for another reason is copied the old way.
func TestCp_aCopyByCopyFile2ThatEndsEarly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("the bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCp(t, dir, func(context.Context, string, string) error { return errorRequestAborted }, "source", "cancelled"); err == nil {
		t.Error("a cancelled copy succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, "cancelled")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a cancelled copy left a file: %v", err)
	}
	refused := func(context.Context, string, string) error { return windows.ERROR_INVALID_PARAMETER }
	if stderr, calls, err := runCp(t, dir, refused, "source", "streamed"); err != nil || calls != 1 {
		t.Fatalf("cp with CopyFile2 refusing: %v, %q, %d", err, stderr, calls)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "streamed")); err != nil || string(data) != "the bytes\n" {
		t.Errorf("the old way's copy holds %q, %v", data, err)
	}
}
