package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A session reads ~/.nemoshrc when ENV is not set at all, as mksh reads ~/.mkshrc, so a shell
// started from a Windows shortcut, which sets no ENV, can be configured with nothing else to
// arrange. ENV, where it is set, is still what is read, as busybox reads it; and an ENV set
// to nothing reads nothing, which is how to ask for no rc file at all.
func TestSession_readsNemoshrcWhenEnvIsNotSet(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".nemoshrc"), []byte("echo NEMOSHRC\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(env, []byte("echo ENV\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	for _, test := range []struct {
		name   string
		env    *string
		args   []string
		stdout string
	}{
		{name: "no ENV reads ~/.nemoshrc", args: []string{"-i", "-c", "echo 2"}, stdout: "NEMOSHRC\n2\n"},
		{name: "ENV is read in its place", env: &env, args: []string{"-i", "-c", "echo 2"}, stdout: "ENV\n2\n"},
		{name: "an empty ENV reads nothing", env: new(""), args: []string{"-i", "-c", "echo 2"}, stdout: "2\n"},
		{name: "--norc reads nothing", args: []string{"--norc", "-i", "-c", "echo 2"}, stdout: "2\n"},
		{name: "a script reads nothing", args: []string{"-c", "echo 2"}, stdout: "2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			t.Setenv("ENV", "")
			if test.env == nil {
				os.Unsetenv("ENV")
			} else {
				os.Setenv("ENV", *test.env)
			}

			// When
			result := runInvocation(t, "", test.args...)

			// Then
			if result.stdout != test.stdout || result.status != 0 {
				t.Fatalf("nemosh %q = %q/%d, want %q/0; stderr %q", test.args, result.stdout, result.status, test.stdout, result.stderr)
			}
		})
	}
}
