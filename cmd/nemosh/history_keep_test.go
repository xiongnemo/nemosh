package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A line HISTCONTROL keeps out of `history` is kept out of the arrows and the history file
// too. ignorespace's line went to both, which is the one place a leading space is meant to
// keep a secret out of.
func TestKeepCommand_keepsWhatHISTCONTROLRefusesOutOfAllThree(t *testing.T) {
	var out bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &out, Stderr: &out})
	rt.RunScript(context.Background(), "HISTCONTROL=ignoreboth\n")
	editor := newLineEditor(strings.NewReader(""), &out, t.TempDir())
	saved := historyFile{path: filepath.Join(t.TempDir(), "h"), limit: 100}

	// When
	for _, command := range []string{"echo kept", " export TOKEN=secret", "echo kept", "echo again"} {
		keepCommand(rt, editor, saved, command)
	}

	// Then
	want := []string{"echo kept", "echo again"}
	if got := rt.HistoryEntries(); !slices.Equal(got, want) {
		t.Fatalf("history = %q, want %q", got, want)
	}
	if got := editor.entries(); !slices.Equal(got, want) {
		t.Fatalf("the arrows walk %q, want %q", got, want)
	}
	data, err := os.ReadFile(saved.path)
	if err != nil || string(data) != "echo kept\necho again\n" {
		t.Fatalf("the history file holds %q, %v; want only the kept lines", data, err)
	}
}
