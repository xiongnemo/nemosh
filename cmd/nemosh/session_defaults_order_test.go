package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A session's defaults -- Ubuntu's ll, la, l, and ls and grep in colour -- come before its
// startup files, so any of those files can take one away or define it otherwise, a login
// shell's profile as much as the rc file: an `alias ll=...` in ~/.profile was replaced by the
// default, which a login session set after the profile had run.
func TestSession_defaultsComeBeforeALoginProfile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".profile"), []byte("alias ll='mine'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PS1", "")
	t.Setenv("PS2", "")
	got := runInvocation(t, "alias ll\n", "-l", "-i")
	if !strings.Contains(got.stdout, "ll='mine'") {
		t.Fatalf("after the profile, alias ll is %q; stderr %q", got.stdout, got.stderr)
	}
}
