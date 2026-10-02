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
	entries []historyEntry
	// lineAdded is whether the command line now running was recorded, which `history -s`
	// and -p take back out, once, as bash's do: the entry is the `history -s` itself.
	lineAdded bool
	// inFile is how many lines of the history file the list has read or written, the
	// place `history -n` reads on from.
	inFile int
}

// historyEntry is a line, and whether the history file has it already, which is what
// `history -a` writes: the lines that are not in it.
type historyEntry struct {
	line  string
	saved bool
}

func newShellHistory() *shellHistory { return &shellHistory{} }

// record adds a line. A blank line and an immediate repeat are skipped, which
// is what the line editor does and what every other shell does; without it the
// arrows fill up with the same command.
func (h *shellHistory) record(line string, saved bool) bool {
	if h == nil || strings.TrimSpace(line) == "" {
		return false
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if len(h.entries) > 0 && h.entries[len(h.entries)-1].line == line {
		return false
	}
	h.entries = append(h.entries, historyEntry{line: line, saved: saved})
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
	}
	return true
}

func (h *shellHistory) list() []string {
	if h == nil {
		return nil
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	listed := make([]string, len(h.entries))
	for index, entry := range h.entries {
		listed[index] = entry.line
	}
	return listed
}

func (h *shellHistory) clear() {
	if h == nil {
		return
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.entries, h.lineAdded = nil, false
}

// RecordHistory adds a line the session read back from its history file at startup,
// which the file has already.
func (r Runtime) RecordHistory(line string) { r.history.record(line, true) }

// HistoryEntries is what has been recorded, oldest first.
//
// Exported for history expansion, which runs in the CLI: `!!` has to mean the same
// line `history` prints, so both read this one list rather than each keeping its own.
func (r Runtime) HistoryEntries() []string { return r.history.list() }

// RunHistoryBuiltin is the builtin, exported so the editor's own list and this
// one cannot diverge in a test.
func (r Runtime) RunHistoryBuiltin(args []string) int { return r.historyBuiltin(args) }

// printHistory prints the newest count entries, all of them when count is negative.
//
// The row is a right-aligned number, two spaces, then the line -- busybox's
// format and bash's. A startup file that pipes `history | grep` depends on the
// command being the tail of the row.
//
// A non-interactive shell has recorded nothing, so this prints nothing rather
// than failing: `history` in a script is a question with a valid answer.
func (r Runtime) printHistory(count int) {
	entries := r.history.list()
	first := 0
	if count >= 0 && count < len(entries) {
		first = len(entries) - count
	}
	width := len(fmt.Sprint(len(entries)))
	for index := first; index < len(entries); index++ {
		fmt.Fprintf(r.streams.Stdout, "%*d  %s\n", width, index+1, entries[index])
	}
}

// erase removes every earlier copy of a line, for HISTCONTROL=erasedups.
func (h *shellHistory) erase(line string) {
	if h == nil {
		return
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	kept := h.entries[:0]
	for _, entry := range h.entries {
		if entry.line != line {
			kept = append(kept, entry)
		}
	}
	h.entries = kept
}

// truncate keeps the newest limit entries, for HISTSIZE.
//
// The newest rather than the oldest, which is the only useful direction: a history that
// dropped what you just ran would make the arrows useless at exactly the moment they are
// wanted.
func (h *shellHistory) truncate(limit int) {
	if h == nil || limit < 0 {
		return
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if len(h.entries) > limit {
		h.entries = append([]historyEntry(nil), h.entries[len(h.entries)-limit:]...)
	}
}

// remove takes out the entries from first to last, counted from 0, and answers whether
// there were such entries to take.
func (h *shellHistory) remove(first, last int) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if first < 0 || last >= len(h.entries) || first > last {
		return false
	}
	h.entries = append(h.entries[:first:first], h.entries[last+1:]...)
	return true
}

// noteLine is the interactive loop's word on the line it is about to run: whether it was
// recorded, and whether the history file has it now, a line on from those counted.
func (h *shellHistory) noteLine(added, written bool) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.lineAdded = added
	if written {
		h.inFile++
	}
}

// takeBackLine removes the entry the running command line made, the first time it is
// asked: `history -s` and -p put their own words where the command that ran them was.
func (h *shellHistory) takeBackLine() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.lineAdded && len(h.entries) > 0 {
		h.entries = h.entries[:len(h.entries)-1]
	}
	h.lineAdded = false
}
