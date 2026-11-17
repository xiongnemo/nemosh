package main

import (
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// setDefaultHistoryFile is HISTFILE at a prompt: once the rc file has run, a session that
// it left without one has it set to $XDG_STATE_HOME/nemosh/history, as busybox's ash sets
// ~/.ash_history after $ENV and bash ~/.bash_history; see xdg_dirs.go for the place, and for
// ~/.nemosh_history, which is used and said when it is the only one there. The variable says
// where history goes, so `echo $HISTFILE` answers and `history -w` with no FILE writes there;
// it was unset, the file used without being named.
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
	path, fromLegacy := resolveHistoryPath(rt.LookupVariable, exists)
	if path == "" {
		return
	}
	rt.SetVariable("HISTFILE", path)
	if fromLegacy {
		current, _ := historyPaths(rt.LookupVariable)
		sayOldLocation(stderr, "history", path, current)
	}
}
