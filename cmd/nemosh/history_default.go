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
//
// In bash mode it is ~/.bash_history, bash's, which Git's bash shares; historySources reads
// ~/.nemosh_history after it.
func setDefaultHistoryFile(rt runtime.Runtime, stderr io.Writer) {
	if _, set := rt.LookupVariable("HISTFILE"); set {
		return
	}
	if rt.BashMode() {
		if path, ok := homeFile(rt, ".bash_history"); ok {
			rt.SetVariable("HISTFILE", path)
		}
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

// historySources are the history files a session reads before its first prompt: the one
// HISTFILE names, and in bash mode ~/.nemosh_history after it, so a line typed under either
// name can be recalled under the other. Lines are written to the first alone.
func historySources(lookup func(string) (string, bool), bashMode bool, toNative func(string) (string, bool)) []historyFile {
	sources := []historyFile{newHistoryFile(lookup, toNative)}
	nemoshs, _ := historyPaths(lookup)
	if histfile, _ := lookup("HISTFILE"); !bashMode || nemoshs == "" || nemoshs == histfile {
		return sources
	}
	return append(sources, newHistoryFile(func(name string) (string, bool) {
		if name == "HISTFILE" {
			return nemoshs, true
		}
		return lookup(name)
	}, toNative))
}
