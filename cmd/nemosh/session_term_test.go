package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A session on Windows is the Windows console's, whatever TERM the environment brought -- Git's
// bash exports xterm-256color, and so may whatever started this shell: the TERM is set aside
// before the rc file, which can set one again, and the session says so when none was. A script
// passes its environment through as any shell does. NEMOSH_DEFAULTS=off keeps the one it had.
func TestSession_setsAsideAnInheritedTERM(t *testing.T) {
	report := `echo "[${TERM-unset}]"; printenv TERM || echo not-exported`
	notice := "nemosh: set aside inherited TERM=xterm-256color; export TERM in ~/.nemoshrc to keep it\n"
	rc := filepath.Join(t.TempDir(), "rc")
	if err := os.WriteFile(rc, []byte("export TERM=xterm-256color\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, defaults string
		args           []string
		stdout         string
		noticed        bool
	}{
		{name: "a session", args: []string{"--norc", "-i", "-c", report}, stdout: "[unset]\nnot-exported\n", noticed: true},
		{name: "an rc file sets one", args: []string{"--rcfile", filepath.ToSlash(rc), "-i", "-c", report},
			stdout: "[xterm-256color]\nxterm-256color\n"},
		{name: "NEMOSH_DEFAULTS=off", defaults: "off", args: []string{"--norc", "-i", "-c", report},
			stdout: "[xterm-256color]\nxterm-256color\n"},
		{name: "a script", args: []string{"-c", report}, stdout: "[xterm-256color]\nxterm-256color\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			if test.defaults != "" {
				t.Setenv("NEMOSH_DEFAULTS", test.defaults)
			}
			result := runInvocation(t, "", test.args...)
			if result.stdout != test.stdout {
				t.Errorf("stdout %q, want %q; stderr %q", result.stdout, test.stdout, result.stderr)
			}
			if got := strings.Contains(result.stderr, notice); got != test.noticed {
				t.Errorf("noticed %v, want %v: stderr %q", got, test.noticed, result.stderr)
			}
		})
	}
}
