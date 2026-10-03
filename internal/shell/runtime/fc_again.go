package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// fcAgain is `fc -s [pat=rep]... [command]`, and `fc -e -`: the newest entry, or the newest
// that COMMAND names, with each pat replaced by its rep wherever it stands, said on standard
// error and run. It takes the newest entry's place in the list, as bash's fc_replhist has it
// do, which at a prompt is the `fc -s` itself.
func (r Runtime) fcAgain(ctx context.Context, operands []string, savedStatus int) lineResult {
	var replacements [][2]string
	for len(operands) > 0 && strings.Contains(operands[0], "=") {
		pattern, replacement, _ := strings.Cut(operands[0], "=")
		replacements = append(replacements, [2]string{pattern, replacement})
		operands = operands[1:]
	}
	entries := r.history.list()
	newest := r.fcNewest(len(entries))
	index, fault := newest, fcFound
	if len(operands) > 0 {
		index, fault = fcEntryNumber(operands[0], entries, newest, len(entries)-1, false, false)
	}
	if fault != fcFound || index < 0 || index >= len(entries) {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"fc: no command found")
		return lineResult{status: 1}
	}
	if r.options.posix && len(operands) > 1 {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"fc: too many arguments")
		return lineResult{status: 1}
	}
	command := entries[index]
	for _, replacement := range replacements {
		if replacement[0] != "" {
			command = strings.ReplaceAll(command, replacement[0], replacement[1])
		}
	}
	fmt.Fprintln(r.streams.Stderr, command)
	r.history.replaceNewest(r, command)
	return r.fcRun(ctx, command, savedStatus)
}

// fcEdit writes lines to a file, runs the editor on it -- -e's, or FCEDIT's, or EDITOR's,
// or vi -- and when the editor succeeds says what the file holds then and runs it, kept in
// the list as one entry. The file is removed either way.
func (r Runtime) fcEdit(ctx context.Context, lines []string, editor string, savedStatus int) lineResult {
	file, err := os.CreateTemp("", "nemosh-fc-*.sh")
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sfc: cannot open temp file: %v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 1}
	}
	path := file.Name()
	defer os.Remove(path)
	_, err = file.WriteString(strings.Join(lines, "\n") + "\n")
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sfc: %v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 1}
	}
	if editor == "" {
		editor = "${FCEDIT:-${EDITOR:-vi}}"
	}
	if status, _ := r.runScriptResult(ctx, editor+" "+singleQuoteForReuse(path), r.currentLine(), false, savedStatus); status != 0 {
		return lineResult{status: 1}
	}
	edited, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sfc: %v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 1}
	}
	text := string(edited)
	fmt.Fprint(r.streams.Stderr, text)
	if command := strings.TrimRight(text, "\r\n"); command != "" {
		r.addHistoryLine(command, false)
	}
	return r.fcRun(ctx, text, savedStatus)
}

// fcRun runs what fc took from history, as eval runs its words: a `return` or an `exit` in
// it leaves what ran fc.
func (r Runtime) fcRun(ctx context.Context, command string, savedStatus int) lineResult {
	status, control := r.runScriptResult(ctx, command, r.currentLine(), false, savedStatus)
	return lineResult{status: status, control: control}
}

// replaceNewest is fc_replhist: the newest entry out, and command in, as HISTCONTROL allows.
func (h *shellHistory) replaceNewest(r Runtime, command string) {
	command = strings.TrimSuffix(command, "\n")
	if command == "" {
		return
	}
	h.mutex.Lock()
	if len(h.entries) > 0 {
		h.entries = h.entries[:len(h.entries)-1]
	}
	h.lineAdded = false
	h.mutex.Unlock()
	r.addHistoryLine(command, false)
}

// lineWasAdded is whether the command line now running was recorded; see lineAdded.
func (h *shellHistory) lineWasAdded() bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.lineAdded
}
