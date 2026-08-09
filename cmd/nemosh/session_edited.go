package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runInteractiveEdited is the session loop for a real terminal.
//
// It is a separate loop rather than a branch inside the cooked one, because the
// two get their lines in incompatible ways. The cooked loop starts a
// cancellable read and then selects on it, so an idle Ctrl-C can interrupt a
// blocked read; the editor reads keys itself and handles Ctrl-C as one of them.
// Threading the editor into that select would leave two readers competing for
// the same stdin.
//
// Everything after a line is obtained -- accumulate, parse, run, report -- is
// the same work, and is shared through runEditedLine.
func (c command) runInteractiveEdited(ctx context.Context, controller *interruptController, editor *lineEditor) (runErr error) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: c.stdin, Stdout: c.stdout, Stderr: c.stderr})
	sourceStartupFile(ctx, rt, c.stderr)

	lastStatus := 0
	var input strings.Builder

	for {
		// Completion follows `cd`, so the directory is refreshed each round
		// rather than captured once when the editor was built. It is asked for
		// in native form: WorkingDirectory answers in the shell's view, which
		// os.ReadDir cannot open on Windows.
		editor.workingDirectory = completionDirectory(rt)
		prompt := interactivePromptWithStatus(ctx, rt, input.Len() > 0, lastStatus)
		line, err := editor.readLine(ctx, prompt)
		if ctx.Err() != nil {
			rt.CloseInteractive(ctx)
			return ctx.Err()
		}
		if errors.Is(err, errLineAbandoned) {
			// Ctrl-C: the partial command goes away and the prompt returns.
			input.Reset()
			continue
		}
		if errors.Is(err, io.EOF) {
			if input.Len() > 0 {
				rt.CloseInteractive(ctx)
				fmt.Fprintln(c.stderr, "nemosh: unexpected end of file")
				return exitStatus(2)
			}
			return interactiveStatusError(rt.CloseInteractive(ctx))
		}
		if err != nil {
			rt.CloseInteractive(ctx)
			return fmt.Errorf("nemosh: read stdin: %w", err)
		}

		appendInteractiveLine(&input, line+"\n")
		script, parseErr := runtime.ParseScript(input.String())
		if errors.Is(parseErr, runtime.ErrIncompleteScript) {
			continue
		}
		// The whole command is remembered, not each physical line, so recalling
		// a multi-line loop brings back the loop. Both lists get it: the arrows
		// walk the editor's and `history` prints the shell's, and a user who
		// saw one would be surprised to find the other different.
		command := strings.TrimRight(input.String(), "\n")
		editor.remember(command)
		rt.RecordHistory(command)
		input.Reset()
		if parseErr != nil {
			rt.ReportInteractiveParseError(parseErr)
			continue
		}
		status, exited, done := c.runEditedLine(ctx, rt, controller, script)
		lastStatus = status
		if done != nil {
			return done
		}
		if exited {
			return interactiveStatusError(status)
		}
	}
}

// runEditedLine executes one parsed command and reports what the loop should do
// next: the status, whether the shell exited, and a terminal error if any.
func (c command) runEditedLine(ctx context.Context, rt runtime.Runtime, controller *interruptController, script runtime.Script) (int, bool, error) {
	executionCtx, clear, interrupted := controller.begin(ctx)
	if interrupted {
		clear()
		fmt.Fprintln(c.stderr)
		return 130, false, nil
	}
	result := rt.RunInteractive(executionCtx, script)
	clear()
	if ctx.Err() != nil {
		rt.CloseInteractive(ctx)
		return result.Status, false, ctx.Err()
	}
	if result.Status == 130 && runtime.IsShellInterrupt(executionCtx) {
		fmt.Fprintln(c.stderr)
	}
	return result.Status, result.Exited, nil
}
