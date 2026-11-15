//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A path is completed in whatever spelling the shell reads, not only in the one Go reads. `cd`
// takes /c/Users and /mnt/c/Users, and Tab after either offered nothing: the directory was
// joined onto the working directory as a relative path, since Go does not call /c/Users
// absolute, and the read failed silently.
func TestLineEditor_completesUnderEverySpellingTheShellReads(t *testing.T) {
	directory := t.TempDir()
	if long, err := filepath.EvalSymlinks(directory); err == nil {
		directory = long
	}
	if err := os.Mkdir(filepath.Join(directory, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	drive := filepath.ToSlash(directory)
	alias := "/" + strings.ToLower(drive[:1]) + drive[2:]
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	for _, typed := range []string{drive, alias, "/mnt" + alias} {
		t.Run(typed[:6], func(t *testing.T) {
			// Given
			screen := newScreenModel(t, 400)
			editor := newLineEditor(strings.NewReader("ls "+typed+"/Docu\t\r"), screen, t.TempDir())
			editor.width = func() int { return 400 }
			editor.resolve = completionResolver(rt)

			// When
			line, err := editor.readLine(context.Background(), "$ ")

			// Then: completed, and in the spelling it was typed in
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if want := "ls " + typed + "/Documents/"; line != want {
				t.Fatalf("line = %q, want %q", line, want)
			}
		})
	}
}
