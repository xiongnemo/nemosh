package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// edit-and-execute-command, readline's C-x C-e: the line being typed goes to a file and the
// file to an editor -- VISUAL, or EDITOR, or emacs, or vi in vi mode, as bash's readline
// chooses -- and what the editor leaves is said on standard error and run as though it had
// been typed, kept in history in the line's place. An editor that fails runs nothing.
// busybox's editor has no such key.

// errEditAndExecute is C-x C-e: the editor answers with the line, to go to an editor.
var errEditAndExecute = errors.New("edit and execute")

// editInEditor hands line to the editor and answers what the editor left, and whether it
// succeeded.
func (c command) editInEditor(ctx context.Context, rt *runtime.Runtime, controller *interruptController, line string, vi bool) (string, bool) {
	file, err := os.CreateTemp("", "nemosh-edit-*.sh")
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: cannot open temp file: %v\n", err)
		return "", false
	}
	path := file.Name()
	defer os.Remove(path)
	_, err = file.WriteString(line + "\n")
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: %v\n", err)
		return "", false
	}
	fallback := "emacs"
	if vi {
		fallback = "vi"
	}
	command := "${VISUAL:-${EDITOR:-" + fallback + "}} '" + strings.ReplaceAll(path, "'", `'\''`) + "'\n"
	script, err := rt.ParseSessionInput(command)
	if err != nil {
		return "", false
	}
	executionCtx, clear, interrupted := controller.begin(ctx)
	if interrupted {
		clear()
		return "", false
	}
	result := rt.RunInteractive(executionCtx, script)
	clear()
	if result.Status != 0 {
		return "", false
	}
	edited, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: %v\n", err)
		return "", false
	}
	text := strings.TrimRight(string(edited), "\r\n")
	if text != "" {
		fmt.Fprintln(c.stderr, text)
	}
	return text, true
}
