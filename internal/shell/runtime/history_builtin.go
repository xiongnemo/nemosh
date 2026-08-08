package runtime

import (
	"fmt"
	"strings"
	"sync"
)

// maxHistoryEntries bounds what an interactive session keeps. busybox's default
// MAX_HISTORY is 999; the same ceiling keeps a long session from growing without
// limit while staying far past what anyone scrolls back through.
const maxHistoryEntries = 999

// shellHistory is the list `history` prints and the arrows walk.
//
// It belongs to the shell rather than to a utility, which is why `history` is a
// builtin and not an applet: a separate process could not see it. busybox makes
// the same call, registering it in the ash builtin table under `#if MAX_HISTORY`.
//
// Shared by pointer across snapshots, so a command recorded in a subshell or a
// pipeline stage is still there when the parent asks.
type shellHistory struct {
	mutex   sync.Mutex
	entries []string
}

func newShellHistory() *shellHistory { return &shellHistory{} }

// record adds a line. A blank line and an immediate repeat are skipped, which
// is what the line editor does and what every other shell does; without it the
// arrows fill up with the same command.
func (h *shellHistory) record(line string) {
	if h == nil || strings.TrimSpace(line) == "" {
		return
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == line {
		return
	}
	h.entries = append(h.entries, line)
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
	}
}

func (h *shellHistory) list() []string {
	if h == nil {
		return nil
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	listed := make([]string, len(h.entries))
	copy(listed, h.entries)
	return listed
}

func (h *shellHistory) clear() {
	if h == nil {
		return
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.entries = nil
}

// RecordHistory adds a line from the interactive loop, which is the only place
// that knows a command was typed rather than read from a script.
func (r Runtime) RecordHistory(line string) { r.history.record(line) }

// RunHistoryBuiltin is the builtin, exported so the editor's own list and this
// one cannot diverge in a test.
func (r Runtime) RunHistoryBuiltin(args []string) int { return r.historyBuiltin(args) }

// historyBuiltin prints the numbered list, or clears it.
//
// The row is a right-aligned number, two spaces, then the line -- busybox's
// format and bash's. A startup file that pipes `history | grep` depends on the
// command being the tail of the row.
//
// A non-interactive shell has recorded nothing, so this prints nothing rather
// than failing: `history` in a script is a question with a valid answer.
func (r Runtime) historyBuiltin(args []string) int {
	for _, arg := range args {
		switch arg {
		case "-c":
			r.history.clear()
			return 0
		default:
			fmt.Fprintf(r.streams.Stderr, "history: unsupported history option: %s; this build implements -c\n", arg)
			return 2
		}
	}
	entries := r.history.list()
	width := len(fmt.Sprint(len(entries)))
	for index, entry := range entries {
		fmt.Fprintf(r.streams.Stdout, "%*d  %s\n", width, index+1, entry)
	}
	return 0
}
