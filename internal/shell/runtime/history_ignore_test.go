package runtime_test

import (
	"context"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// HISTIGNORE keeps a line out of the history, the file and the arrows when one of its
// patterns matches the whole line: a colon-separated list of shell patterns, & standing for
// the line before, \: for a colon. It was set and had no effect, as HISTCONTROL once had.
// The same lines in bash 5.3 keep exactly what this keeps.
func TestRuntime_historyIgnoresWhatHistignoreNames(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	rt.RunScript(context.Background(), `HISTIGNORE='ls*:&:ech?\:x:  *'`+"\n")

	// When
	for _, line := range []string{"echo a", "ls -l", "echo a", "echo b", "echo:x", "  echo indented", "echo c"} {
		rt.RecordInteractiveLine(line, false)
	}

	// Then: ls by its pattern, the second `echo a` as a repeat of the line before it -- the
	// ls was never kept -- echo:x by the escaped colon, and the indented line by its blanks
	if got, want := rt.HistoryEntries(), []string{"echo a", "echo b", "echo c"}; !slices.Equal(got, want) {
		t.Fatalf("history = %q, want %q", got, want)
	}
}
