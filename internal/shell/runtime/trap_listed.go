package runtime

// A subshell does not run the shell's DEBUG and ERR traps unless `set -T` or `set -E` passes
// them on (inheritedTraps), but it lists them, as bash's does: `$(trap -p DEBUG)` is the
// shell's trap, which is how starship and bash-preexec find one already there to run beside
// theirs, and `saved=$(trap)` keeps them. One the subshell sets or resets is its own from then
// on. A function hides DEBUG as bash does, and lists none (hideDebugTrap). busybox lists no
// trap at all in a subshell; the signal traps were listed here already, as bash lists them.

// droppedTraps are the traps a snapshot of r lists without running: those inheritedTraps
// leaves out, and those r itself was listing so.
func (r Runtime) droppedTraps(inherited map[trapName]string) map[trapName]string {
	dropped := map[trapName]string{}
	for name, action := range r.listedTraps {
		dropped[name] = action
	}
	for name, action := range r.traps {
		if _, kept := inherited[name]; !kept {
			dropped[name] = action
		}
	}
	return dropped
}

// shownTraps is every trap `trap` and `trap -p` list: the shell's own, and the ones it lists
// without running.
func (r Runtime) shownTraps() map[trapName]string {
	if len(r.listedTraps) == 0 {
		return r.traps
	}
	shown := cloneMap(r.listedTraps)
	for name, action := range r.traps {
		shown[name] = action
	}
	return shown
}
