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

func (c command) runInteractive(ctx context.Context, controller *interruptController) (runErr error) {
	inputReader := newInteractiveInput(c.stdin)
	defer func() { runErr = errors.Join(runErr, inputReader.close()) }()
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: inputReader, Stdout: c.stdout, Stderr: c.stderr})
	sourceStartupFile(ctx, rt, c.stderr)
	idleInterrupts := controller.idleInterrupts()
	var lineResults <-chan interactiveLine
	linePending := false
	lastStatus := 0
	var input strings.Builder

sessionLoop:
	for {
		if !linePending {
			accumulated := input.Len()
			linePending = true
			lineResults = inputReader.readLine(ctx, accumulated)
		}
		fmt.Fprint(c.stderr, interactivePromptWithStatus(ctx, rt, input.Len() > 0, lastStatus))
		var lineResult interactiveLine
		for {
			if !linePending {
				accumulated := input.Len()
				linePending = true
				lineResults = inputReader.readLine(ctx, accumulated)
			}
			select {
			case <-ctx.Done():
				inputReader.reset()
				linePending = false
				rt.CloseInteractive(ctx)
				return ctx.Err()
			case <-idleInterrupts:
				if !controller.consumeIdleInterrupt() {
					continue
				}
				inputReader.reset()
				linePending = false
				input.Reset()
				fmt.Fprintln(c.stderr)
				continue sessionLoop
			case lineResult = <-lineResults:
				linePending = false
				if controller.consumeIdleInterrupt() {
					inputReader.reset()
					input.Reset()
					fmt.Fprintln(c.stderr)
					continue sessionLoop
				}
			}
			break
		}
		line, err := lineResult.text, lineResult.err
		if ctx.Err() != nil {
			rt.CloseInteractive(ctx)
			return ctx.Err()
		}
		if errors.Is(err, errInputTooLarge) {
			rt.CloseInteractive(ctx)
			fmt.Fprintln(c.stderr, "nemosh: input too large")
			return exitStatus(2)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			rt.CloseInteractive(ctx)
			return fmt.Errorf("nemosh: read stdin: %w", err)
		}
		if line == "" && errors.Is(err, io.EOF) {
			if input.Len() > 0 {
				rt.CloseInteractive(ctx)
				fmt.Fprintln(c.stderr, "nemosh: unexpected end of file")
				return exitStatus(2)
			}
			return interactiveStatusError(rt.CloseInteractive(ctx))
		}
		appendInteractiveLine(&input, line)
		script, parseErr := runtime.ParseScript(input.String())
		if errors.Is(parseErr, runtime.ErrIncompleteScript) {
			if errors.Is(err, io.EOF) {
				rt.CloseInteractive(ctx)
				fmt.Fprintln(c.stderr, "nemosh: unexpected end of file")
				return exitStatus(2)
			}
			continue
		}
		input.Reset()
		if parseErr != nil {
			rt.ReportInteractiveParseError(parseErr)
			if errors.Is(err, io.EOF) {
				return interactiveStatusError(rt.CloseInteractive(ctx))
			}
			continue
		}
		executionCtx, clear, interrupted := controller.begin(ctx)
		if interrupted {
			clear()
			inputReader.reset()
			input.Reset()
			fmt.Fprintln(c.stderr)
			continue
		}
		result := rt.RunInteractive(executionCtx, script)
		lastStatus = result.Status
		clear()
		if ctx.Err() != nil {
			rt.CloseInteractive(ctx)
			return ctx.Err()
		}
		if result.Status == 130 && runtime.IsShellInterrupt(executionCtx) {
			inputReader.reset()
			fmt.Fprintln(c.stderr)
		}
		if result.Exited {
			return interactiveStatusError(result.Status)
		}
		if errors.Is(err, io.EOF) {
			return interactiveStatusError(rt.CloseInteractive(ctx))
		}
	}
}
