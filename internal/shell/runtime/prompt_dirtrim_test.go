package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// PROMPT_DIRTRIM is bash's: \w keeps that many trailing directories and puts ... for the rest,
// after a ~/ at the front -- bash's trim_pathname, which leaves a path alone when what it would
// take out is three characters or fewer, so ~/x/y/z/w trimmed to 2 is itself, as bash 5.3 drew
// it on 2026-11-21, and ~/a/b/c/d/e is ~/.../d/e. \W is not trimmed. busybox has no such
// variable, so it is bash's under every name.
func TestPrompt_PROMPT_DIRTRIM(t *testing.T) {
	home := t.TempDir()
	other := t.TempDir()
	for _, dir := range []string{filepath.Join(home, "x", "y", "z", "w"), filepath.Join(home, "a", "b", "c", "d", "e"), filepath.Join(other, "q", "r", "s")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	slashed := func(path string) string { return filepath.ToSlash(path) }
	script := "HOME='" + slashed(home) + "'\np='\\w'; q='\\W'\nPROMPT_DIRTRIM=2\n" +
		"cd '" + slashed(home) + "/x/y/z/w'; echo \"${p@P}\"\n" +
		"cd '" + slashed(home) + "/a/b/c/d/e'; echo \"${p@P} ${q@P}\"\n" +
		"cd '" + slashed(other) + "/q/r/s'; echo \"${p@P}\"\n" +
		"PROMPT_DIRTRIM=0; echo \"${p@P}\"\n"
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("exited %d", status)
	}
	var full bytes.Buffer
	check := New(applets.DefaultRegistry, Streams{Stdout: &full, Stderr: new(bytes.Buffer)})
	check.RunScript(context.Background(), "cd '"+slashed(other)+"/q/r/s'; pwd\n")
	want := "~/x/y/z/w\n~/.../d/e e\n.../r/s\n" + full.String()
	if stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}
