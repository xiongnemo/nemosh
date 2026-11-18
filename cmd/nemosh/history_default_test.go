package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withoutHISTFILE runs a test with HISTFILE unset and HOME a directory of its own.
func withoutHISTFILE(t *testing.T) string {
	t.Helper()
	if old, ok := os.LookupEnv("HISTFILE"); ok {
		os.Unsetenv("HISTFILE")
		t.Cleanup(func() { os.Setenv("HISTFILE", old) })
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// At a prompt HISTFILE is set once the rc file has run, as busybox's ash and bash set
// theirs, so `echo $HISTFILE` answers and `history -w` has a file to write; a script's is
// left unset, as in both.
func TestHistoryFile_isNamedAtAPromptAndNotInAScript(t *testing.T) {
	home := withoutHISTFILE(t)

	// When
	got := runInteractiveTest(strings.NewReader("echo \"[$HISTFILE]\"\n"))
	var script bytes.Buffer
	cmd := command{stdin: strings.NewReader(""), stdout: &script, stderr: &script}
	_ = cmd.run(context.Background(), []string{"nemosh", "-c", `echo "[${HISTFILE-unset}]"`})

	// Then
	// HOME as it is given, so the separator before the base is the host's, and the one after
	// is a slash, as busybox's `C:\Users\me/.ash_history` has it.
	if want := filepath.Base(home) + "/.nemosh_history]\n"; !strings.HasPrefix(got.stdout, "[") || !strings.HasSuffix(got.stdout, want) {
		t.Fatalf("at a prompt HISTFILE is %q, want it under HOME, ending %q", got.stdout, want)
	}
	if script.String() != "[unset]\n" {
		t.Fatalf("in a script HISTFILE is %q, want it unset", script.String())
	}
}

// An rc file's HISTFILE is kept, exported or not.
func TestHistoryFile_keepsTheRCFilesOwn(t *testing.T) {
	home := withoutHISTFILE(t)
	rc := filepath.Join(home, "rc")
	if err := os.WriteFile(rc, []byte("HISTFILE=~/mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV", filepath.ToSlash(rc))

	// When
	got := runInteractiveTest(strings.NewReader("echo \"[$HISTFILE]\"\n"))

	// Then
	if !strings.HasSuffix(got.stdout, "/mine]\n") {
		t.Fatalf("HISTFILE is %q, want the rc file's", got.stdout)
	}
}
