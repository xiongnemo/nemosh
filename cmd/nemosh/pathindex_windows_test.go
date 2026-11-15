//go:build windows

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A PATH entry is read as the launcher reads it: through the path model, so an entry written
// /c/tools is a directory the index can list, as it is one the shell can run from. Go read it
// as \c\tools on the current drive, found nothing, and drew every program in it red.
func TestPathIndex_readsAnEntryInTheShellsSpelling(t *testing.T) {
	// Given
	directory := filepath.ToSlash(seedPathDirectory(t, "tool.exe"))
	alias := "/" + strings.ToLower(directory[:1]) + directory[2:]
	index := newPathIndex()

	// When
	index.refreshWith(alias, completionResolver(runtime.New(applets.DefaultRegistry, runtime.Streams{})))
	waitForPathIndex(t, index)

	// Then
	if found, _ := index.has("tool"); !found {
		t.Fatalf("%s/tool.exe was not indexed", alias)
	}
}

// And a name runs by the same suffixes it is offered by. The launcher tries .sh, so `deploy`
// runs deploy.sh, and the index did not, so `dep<TAB>` offered nothing and `deploy` was red.
func TestPathIndex_offersEverySuffixTheLauncherTries(t *testing.T) {
	// Given
	index := settledIndex(t, seedPathDirectory(t, "deploy.sh", "tool.com", "run.cmd"))

	// Then
	for _, name := range []string{"deploy", "tool", "run"} {
		if !slices.Contains(index.candidates(), name) {
			t.Errorf("candidates = %v, missing %q", index.candidates(), name)
		}
	}
	if found, _ := index.has("deploy.sh"); !found {
		t.Error("deploy.sh is not recognised by its full name")
	}
}
