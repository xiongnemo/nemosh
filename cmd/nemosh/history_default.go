package main

import (
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// setDefaultHistoryFile is HISTFILE at a prompt: once the rc file has run, a session that
// it left without one has it set to ~/.nemosh_history, as busybox's ash sets
// ~/.ash_history after $ENV and bash ~/.bash_history. Where a nightly 1.5.2 or 1.5.3 left a
// history file in the XDG state directory and there is none in the home, that one is used and
// said (xdg_dirs.go). The variable says where history goes, so `echo $HISTFILE` answers and
// `history -w` with no FILE writes there; it was unset, the file used without being named.
func setDefaultHistoryFile(rt runtime.Runtime, stderr io.Writer) {
	if _, set := rt.LookupVariable("HISTFILE"); set {
		return
	}
	exists := func(path string) bool {
		native, ok := nativePath(rt, path)
		if !ok {
			return false
		}
		info, err := os.Stat(native)
		return err == nil && !info.IsDir()
	}
	path, fromNightly := resolveHistoryPath(rt.LookupVariable, exists)
	if path == "" {
		return
	}
	rt.SetVariable("HISTFILE", path)
	if fromNightly {
		current, _ := historyPaths(rt.LookupVariable)
		sayOldLocation(stderr, "history", path, current)
	}
}
