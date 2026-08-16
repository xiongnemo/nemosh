package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
}

// newHistoryFile reads the two variables that decide this. Both are consulted
// through the shell's own environment rather than the process's, so `export
// HISTFILE=...` in an rc file takes effect.
func newHistoryFile(lookup func(string) (string, bool)) historyFile {
	file := historyFile{limit: defaultHistoryEntries}
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
	if !h.enabled() {
		return nil
	}
	file, err := os.Open(h.path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []string
	scan := bufio.NewScanner(file)
	// A long pasted command is still a command. The default 64k token limit
	// would drop it silently, which is the sort of loss nobody traces back.
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scan.Scan() {
		if line := scan.Text(); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > h.limit {
		lines = lines[len(lines)-h.limit:]
	}
	return lines
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
	// would make one entry read back as several, so it is refused rather than
	// corrupted -- recalling half a loop is worse than not recalling it.
	if strings.ContainsAny(line, "\n\r") {
		return
	}
	file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = file.WriteString(line + "\n")
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
	lines := h.readAll()
	if len(lines) <= h.limit*trimAtMultiple {
		return
	}
	kept := lines[len(lines)-h.limit:]
	temporary := h.path + ".new"
	if err := os.WriteFile(temporary, []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
		return
	}
	if err := os.Rename(temporary, h.path); err != nil {
		_ = os.Remove(temporary)
	}
}

// readAll is load without the limit, which trimming needs: it has to see how
// much is there before deciding what to keep.
func (h historyFile) readAll() []string {
	file, err := os.Open(h.path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []string
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scan.Scan() {
		if line := scan.Text(); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
