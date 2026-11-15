package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// History that survives the window closing.
//
// Until now it lived in memory and went away with the session, which made the
// inline suggestion far weaker than it looks: the first source it consults is
// what you have already run, so every new window started with nothing to say and
// fell back to completing command names.
//
// The shape is busybox's, and its reasons are good ones:
//
//   - Appended a line at a time, `O_APPEND`, in one write -- "we (try to) do
//     atomic write" (libbb/lineedit.c:1826). Writing on exit instead would lose
//     the session that crashed, and would have two windows overwrite each other.
//   - `HISTFILE=""` disables saving entirely. bash compatibility, and busybox
//     spells the same special case (`:1819`).
//   - The file is trimmed lazily, when it has grown to several times the limit
//     rather than on every write (`:1841`). Rewriting a file per command to keep
//     it at exactly N lines is work nobody asked for.
//   - A file that cannot be opened does not clear what is already in memory:
//     "NB: do not trash old history if file can't be opened" (`:1697`).
const (
	// defaultHistoryEntries is bash's number. busybox's ceiling is lower because
	// it keeps every entry in a fixed array; nothing here does.
	defaultHistoryEntries = 500
	// trimAtMultiple is when the file is rewritten: busybox's rule, four times
	// the limit, so trimming is rare and each write stays one append.
	trimAtMultiple = 4
)

// historyFile is where history is kept, and how much of it.
type historyFile struct {
	path  string
	limit int
	// lookup reads the shell's variables at the moment of writing, for HISTTIMEFORMAT, which
	// has each command written with its time. Nil writes none.
	lookup func(string) (string, bool)
}

// newHistoryFile reads the two variables that decide this. Both are consulted
// through the shell's own environment rather than the process's, so `export
// HISTFILE=...` in an rc file takes effect.
// toNative crosses from the shell's path spelling to the host's, and may be nil
// where the two are already the same -- a test working in t.TempDir, or any
// platform where there is only one spelling.
//
// It is needed because the launch boundary now translates rather than the
// variable being stored translated: HOME is `/c/Users/nemo` inside the shell,
// and os.OpenFile cannot open that. Handing it over unresolved would have
// written the history to `C:\c\Users\nemo\.nemosh_history`, which is a real
// path, in the wrong place, that nobody would ever look at.
func newHistoryFile(lookup func(string) (string, bool), toNative func(string) (string, bool)) historyFile {
	file := historyFile{limit: defaultHistoryEntries, lookup: lookup}
	if value, ok := lookup("HISTFILESIZE"); ok {
		if size, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && size >= 0 {
			file.limit = size
		}
	}
	value, ok := lookup("HISTFILE")
	switch {
	case ok:
		// Set and empty means off, which is bash's rule and busybox's. An
		// explicit empty value is a decision, not an oversight.
		file.path = value
	default:
		file.path = defaultHistoryPath(lookup)
	}
	if file.path != "" && toNative != nil {
		native, ok := toNative(file.path)
		if !ok {
			// A HISTFILE that does not resolve saves nothing, rather than
			// creating a file somewhere surprising and reporting success.
			return historyFile{limit: file.limit, lookup: lookup}
		}
		file.path = native
	}
	return file
}

func defaultHistoryPath(lookup func(string) (string, bool)) string {
	home, ok := lookup("HOME")
	if !ok || home == "" {
		return ""
	}
	return filepath.Join(home, ".nemosh_history")
}

// enabled reports whether anything is saved at all.
func (h historyFile) enabled() bool { return h.path != "" && h.limit > 0 }

// load returns the remembered lines, oldest first, and never more than the
// limit.
//
// An unreadable file is not an error worth a word: a first run has no file, and
// a caller that cannot read one keeps whatever it already had.
func (h historyFile) load() []string {
	records, _ := h.loadCounted()
	return recordLines(records)
}

// loadCounted is load with each entry's time, and how many entries the file holds, the
// place `history -n` reads on from. The file is read in bash's format, so a # and digits
// is a time and not a command; see runtime.ParseHistoryFile.
func (h historyFile) loadCounted() ([]runtime.HistoryRecord, int) {
	if !h.enabled() {
		return nil, 0
	}
	records := h.readAll()
	held := len(records)
	if len(records) > h.limit {
		records = records[len(records)-h.limit:]
	}
	return records, held
}

func recordLines(records []runtime.HistoryRecord) []string {
	if len(records) == 0 {
		return nil
	}
	lines := make([]string, len(records))
	for index, record := range records {
		lines[index] = record.Line
	}
	return lines
}

// stamped is whether HISTTIMEFORMAT has commands written with their times, as bash has it:
// set, even to nothing.
func (h historyFile) stamped() bool {
	if h.lookup == nil {
		return false
	}
	_, set := h.lookup("HISTTIMEFORMAT")
	return set
}

// writes is whether append would write line: a file to write to, and a line that is one.
// A command of several lines only with times, which say where it ends.
func (h historyFile) writes(line string) bool {
	return h.enabled() && strings.TrimSpace(line) != "" && (h.stamped() || !strings.ContainsAny(line, "\n\r"))
}

// append adds one line and trims the file if it has grown far past the limit.
//
// The line is written with its newline in a single Write, which is what makes
// two shells appending at once interleave whole lines rather than halves.
func (h historyFile) append(line string) {
	if !h.enabled() || strings.TrimSpace(line) == "" {
		return
	}
	// A command spanning several lines is stored as it was typed. Writing it raw
	// with no time before it would make one entry read back as several, so then it
	// is refused rather than corrupted -- recalling half a loop is worse than not
	// recalling it. Under HISTTIMEFORMAT the time before it says where it ends.
	if !h.writes(line) {
		return
	}
	file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = file.WriteString(runtime.FormatHistoryRecord(runtime.HistoryRecord{Line: line}, h.stamped()))
	file.Close()
	h.trimIfLong()
}

// trimIfLong rewrites the file only once it has grown to several times the
// limit, then replaces it in one rename so a reader never sees a partial file.
func (h historyFile) trimIfLong() {
	info, err := os.Stat(h.path)
	if err != nil || info.Size() == 0 {
		return
	}
	records := h.readAll()
	if len(records) <= h.limit*trimAtMultiple {
		return
	}
	var kept strings.Builder
	for _, record := range records[len(records)-h.limit:] {
		// Each with its time if it had one, so the file is read back as it was.
		kept.WriteString(runtime.FormatHistoryRecord(record, !record.When.IsZero()))
	}
	temporary := h.path + ".new"
	if err := os.WriteFile(temporary, []byte(kept.String()), 0o600); err != nil {
		return
	}
	if err := os.Rename(temporary, h.path); err != nil {
		_ = os.Remove(temporary)
	}
}

// readAll is load without the limit, which trimming needs: it has to see how
// much is there before deciding what to keep. Read whole, so a long pasted command
// is still a command; a scanner's token limit dropped one silently.
func (h historyFile) readAll() []runtime.HistoryRecord {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return nil
	}
	return runtime.ParseHistoryFile(string(data))
}
