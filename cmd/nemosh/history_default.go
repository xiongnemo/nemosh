package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// setDefaultHistoryFile is HISTFILE at a prompt: once the rc file has run, a session that
// it left without one has it set to ~/.nemosh_history, as busybox's ash sets
// ~/.ash_history after $ENV and bash ~/.bash_history. The variable says where history
// goes, so `echo $HISTFILE` answers and `history -w` with no FILE writes there; it was
// unset, the file used without being named.
func setDefaultHistoryFile(rt runtime.Runtime) {
	if _, set := rt.LookupVariable("HISTFILE"); set {
		return
	}
	if home, ok := rt.LookupVariable("HOME"); ok && home != "" {
		rt.SetVariable("HISTFILE", strings.TrimRight(home, `/\`)+"/.nemosh_history")
	}
}
