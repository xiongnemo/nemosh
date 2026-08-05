package runtime

import (
	"context"
	"errors"
	"fmt"
)

func (r Runtime) executeCommandNode(ctx context.Context, command commandNode, savedStatus int) lineResult {
	switch value := command.(type) {
	case simpleCommand:
		return r.executeSimpleCommand(ctx, value, savedStatus)
	case braceGroup:
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, false)
	case subshellCommand:
		return r.executeCompoundCommand(ctx, value.body, value.redirects, savedStatus, true)
	default:
		return lineResult{status: 2}
	}
}

func (r Runtime) executeCompoundCommand(ctx context.Context, body Script, redirects []redirectOperation, savedStatus int, isolated bool) lineResult {
	commandRuntime := r
	if isolated {
		var err error
		commandRuntime, err = r.snapshot(ctx)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
			return lineResult{status: 1}
		}
		defer func() {
			commandRuntime.jobScope.cancelAndDrain()
			_ = commandRuntime.fds.closeAll()
		}()
	}
	return commandRuntime.executeWithRedirects(ctx, redirects, savedStatus, func(redirected Runtime) lineResult {
		status, control := redirected.executeProgram(ctx, body.program, savedStatus)
		if isolated {
			control = flowNone
		}
		return lineResult{status: status, control: control}
	})
}

func (r Runtime) executeWithRedirects(ctx context.Context, operations []redirectOperation, savedStatus int, run func(Runtime) lineResult) lineResult {
	operations, ok := r.expandRedirectOperations(ctx, cloneRedirects(operations), savedStatus)
	if !ok {
		return lineResult{status: 1}
	}
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, table.closeAll()))
		return lineResult{status: 1}
	}
	result := run(r.withFDTable(table))
	if err := table.closeAll(); err != nil && result.status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		result.status = 1
	}
	return result
}
