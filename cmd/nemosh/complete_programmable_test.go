package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Tab asks the shell's `complete` specifications first, and puts their answer in as readline
// puts bash's: one candidate whole and a blank after it; as it is unless -o filenames says it
// is a name, which is then quoted; with no blank under -o nospace; several put in as far as
// they agree; and nothing answered completes files when -o default asks for it.
func TestComplete_asksTheShellsSpecificationsFirst(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: io.Discard, Stderr: io.Discard})
	rt.SetInvocationMode("is")
	script := `complete -W 'alpha beta alpaca' w
one() { COMPREPLY=('one two'); }
complete -o nospace -F one nsp
complete -o filenames -W "'my file'" fn
none() { COMPREPLY=(); }
complete -o default -F none d
complete -F none bare
`
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("setting up: status %d", status)
	}
	for _, test := range []struct{ typed, want string }{
		{"w b", "w beta "},
		{"w a", "w alp"},
		{"w alph", "w alpha "},
		{"nsp ", "nsp one two"},
		{"fn m", `fn my\ file `},
		{"d no", "d notes.txt "},
		{"bare no", "bare no"},
	} {
		t.Run(test.typed, func(t *testing.T) {
			_, editor := newStyledEditor(t, 80, "", nil)
			editor.workingDirectory = directory
			editor.programmable = func(line string, point int) (runtime.ProgrammableCompletion, bool) {
				return rt.CompleteLine(context.Background(), line, point)
			}
			for _, r := range test.typed {
				editor.buffer.insert(r)
			}
			editor.complete("$ ")
			if got := editor.buffer.String(); got != test.want {
				t.Errorf("buffer = %q, want %q", got, test.want)
			}
		})
	}
}
