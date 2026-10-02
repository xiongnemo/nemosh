package main

import "github.com/xiongnemo/nemosh/internal/shell/runtime"

// keepCommand keeps a command typed at a prompt in the three places a command is kept: the
// shell's list, which `history` prints; the editor's, which the arrows walk; and the
// history file. HISTCONTROL decides for all three, as bash's one list has it decide, so a
// line it keeps out -- ` export TOKEN=...` under ignorespace, a repeat under ignoredups --
// is in none of them. It decided for the first alone, and the secret the leading space was
// to keep out went to the file and back up the arrows.
func keepCommand(rt runtime.Runtime, editor *lineEditor, saved historyFile, command string) {
	if rt.RecordInteractiveLine(command, saved.writes(command)) {
		editor.remember(command)
		// Written now rather than at exit: a session that is killed still leaves what it
		// ran, and two windows appending interleave whole lines instead of overwriting
		// each other.
		saved.append(command)
	}
}
