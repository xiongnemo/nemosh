package applets_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// firstKilobyte copies the start of a system file, which is all the sniff reads.
func firstKilobyte(t *testing.T, from, to string) {
	t.Helper()
	source, err := os.Open(from)
	if err != nil {
		t.Skipf("no %s here: %v", from, err)
	}
	defer source.Close()
	head := make([]byte, 1024)
	n, err := io.ReadFull(source, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, head[:n], 0o644); err != nil {
		t.Fatal(err)
	}
}

// test -x is busybox-w32's execute bit: a directory, a name ending .com .exe .sh .bat or .cmd, or
// a file that begins #! or is a program, and not a DLL whatever it is called. Each answer was
// measured against busybox-w32. It went by four of the suffixes alone.
func TestTestX_isBusyboxW32sExecuteBit(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"script": "#!/bin/sh\necho hi\n", "notes": "plain text\n",
		"run.sh": "x", "fake.exe": "MZ", "mz": "MZab"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	system := os.Getenv("SystemRoot")
	firstKilobyte(t, filepath.Join(system, "System32", "whoami.exe"), filepath.Join(dir, "who"))
	firstKilobyte(t, filepath.Join(system, "System32", "kernel32.dll"), filepath.Join(dir, "k32"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The sniff holds the access time, as busybox's does, so asking does not make it look read.
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "script"), old, old); err != nil {
		t.Fatal(err)
	}
	applet, _ := applets.DefaultRegistry.Lookup("test")
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	for name, want := range map[string]bool{"script": true, "notes": false, "run.sh": true, "fake.exe": true,
		"mz": false, "who": true, "k32": false, "sub": true} {
		err := applet.Run(ctx, []string{"-x", name}, strings.NewReader(""), io.Discard, io.Discard)
		if got := err == nil; got != want {
			t.Errorf("test -x %s: %v (%v), want %v", name, got, err, want)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "script"))
	if err != nil {
		t.Fatal(err)
	}
	accessed := time.Unix(0, info.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if !accessed.Equal(old) {
		t.Errorf("test -x script moved its access time to %v from %v", accessed, old)
	}
}
