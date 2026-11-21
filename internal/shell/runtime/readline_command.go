package runtime

import (
	"context"
	"strconv"
	"unicode/utf8"
)

// RunKeyCommand is keys bind -x bound: command runs in the session with READLINE_LINE the line,
// READLINE_POINT the cursor and READLINE_MARK the mark, and what it leaves in the first two is
// answered, as bash's bash_execute_unix_command does; all three are unset after it, as bash 5
// unsets them. The point counts characters, as ${READLINE_LINE:0:READLINE_POINT} reads it, and
// is read as an integer is, so fzf's 0x7fffffff, for the end of the line, is taken.
func (r *Runtime) RunKeyCommand(ctx context.Context, command, line string, point int) (string, int) {
	r.vars["READLINE_LINE"] = line
	r.vars["READLINE_POINT"] = strconv.Itoa(point)
	r.vars["READLINE_MARK"] = "0"
	r.runScriptResult(ctx, command, r.currentLine(), false, r.interactive.status)
	line = r.vars["READLINE_LINE"]
	if moved, err := strconv.ParseInt(r.vars["READLINE_POINT"], 0, 64); err == nil {
		point = int(min(max(moved, 0), int64(utf8.RuneCountInString(line))))
	}
	for _, name := range [...]string{"READLINE_LINE", "READLINE_POINT", "READLINE_MARK"} {
		r.unsetName(name)
	}
	return line, point
}

// ExpandLine is readline's shell-expand-line, M-C-e: the line with its parameters, command
// substitutions and arithmetic expanded as the inside of double quotes is, so a quote of its
// own stays where bash's expansion would take it away. fzf's Alt-C puts `__fzf_cd__` in
// backquotes into the line and expands it so before it accepts the line. An expansion that
// fails is said, and leaves the line as it was.
func (r Runtime) ExpandLine(ctx context.Context, line string) (string, bool) {
	expanded := r.expandPromptText(ctx, line, r.interactive.status, false)
	if r.promptExpansionFailed() {
		return line, false
	}
	return expanded, true
}
