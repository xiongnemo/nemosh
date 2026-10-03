package runtime

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// historyExpand is history expansion, which the CLI has and lends: `history -p` expands
// its words against the list as `!!` at a prompt does. Nil where nothing has lent it, as
// in this package's own tests, and then a word is printed as it is.
var historyExpand func(line string, entries []string) (string, error)

// SetHistoryExpander lends the runtime the CLI's history expansion, for `history -p`.
func SetHistoryExpander(expand func(line string, entries []string) (string, error)) {
	historyExpand = expand
}

// NoteHistoryFile is how many lines the session read from its history file at startup,
// the place `history -n` reads on from.
func (r Runtime) NoteHistoryFile(lines int) {
	r.history.mutex.Lock()
	defer r.history.mutex.Unlock()
	r.history.inFile = lines
}

// printHistoryExpansions is -p: each word expanded and printed, and none of it kept. The
// `history -p` itself goes too, so that `!!` is the command before it.
func (r Runtime) printHistoryExpansions(words []string) int {
	r.history.takeBackLine()
	status := 0
	for _, word := range words {
		expanded := word
		if historyExpand != nil {
			var err error
			if expanded, err = historyExpand(word, r.history.list()); err != nil {
				fmt.Fprintf(r.streams.Stderr, "%shistory: %s: history expansion failed\n", r.diagnosticPrefix(), word)
				status = 1
				continue
			}
		}
		fmt.Fprintln(r.streams.Stdout, expanded)
	}
	return status
}

// historyFileRequest is -a, -n, -r and -w, on the FILE given or on HISTFILE. A file that
// cannot be read or written fails quietly, status 1, as bash's does.
func (r Runtime) historyFileRequest(request historyRequest, operands []string) int {
	name, given := r.vars["HISTFILE"], len(operands) > 0
	if given {
		name = operands[0]
	}
	if name == "" {
		if given {
			fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"history: empty filename")
		} else {
			fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"history: HISTFILE: parameter null or not set")
		}
		if strings.ContainsRune(r.options.invocation, 'i') {
			return 0
		}
		return 1
	}
	path := r.resolvePath(name)
	if path == "" {
		return 1
	}
	switch {
	case request.has('a'):
		return r.appendHistoryFile(name, path)
	case request.has('w'):
		return r.writeHistoryFile(path)
	case request.has('r'), request.has('n'):
		return r.readHistoryFile(path, request.has('n'))
	}
	return 0
}

// appendHistoryFile is -a: the lines the file does not have yet. With none, a file that
// is not there is not made.
func (r Runtime) appendHistoryFile(name, path string) int {
	lines := r.history.unsavedLines()
	if len(lines) == 0 {
		return 0
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, r.createMode(0o600))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(r.streams.Stderr, "%shistory: %s: cannot create: %s\n", r.diagnosticPrefix(), name, applets.CauseText(err))
		}
		return 1
	}
	text := strings.Join(lines, "\n") + "\n"
	_, err = file.WriteString(text)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 1
	}
	r.history.markSaved(false, strings.Count(text, "\n"))
	return 0
}

// writeHistoryFile is -w: the whole list, in place of what the file held.
func (r Runtime) writeHistoryFile(path string) int {
	var text strings.Builder
	for _, line := range r.history.list() {
		text.WriteString(line + "\n")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, r.createMode(0o600))
	if err != nil {
		return 1
	}
	_, err = file.WriteString(text.String())
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 1
	}
	r.history.markSaved(true, strings.Count(text.String(), "\n"))
	return 0
}

// readHistoryFile is -r, every line of the file added to the list, and -n, only the lines
// past those the list has read or written. -n's count as this session's, which -a then
// writes, as bash counts them.
func (r Runtime) readHistoryFile(path string, onlyNew bool) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 1
	}
	var lines []string
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scan.Scan() {
		if line := scan.Text(); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	r.history.mutex.Lock()
	from := 0
	if onlyNew {
		from = min(r.history.inFile, len(lines))
	}
	for _, line := range lines[from:] {
		r.history.entries = append(r.history.entries, historyEntry{line: line, saved: !onlyNew})
	}
	r.history.inFile = len(lines)
	r.history.mutex.Unlock()
	r.history.truncate(r.historyLimit())
	return 0
}

// unsavedLines is what -a writes.
func (h *shellHistory) unsavedLines() []string {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	var lines []string
	for _, entry := range h.entries {
		if !entry.saved {
			lines = append(lines, entry.line)
		}
	}
	return lines
}

// markSaved records that the file has every entry now, and holds lines lines more, or
// holds only them when it was written whole.
func (h *shellHistory) markSaved(whole bool, lines int) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	for index := range h.entries {
		h.entries[index].saved = true
	}
	if whole {
		h.inFile = 0
	}
	h.inFile += lines
}
