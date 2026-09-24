package main

import (
	"context"
	"fmt"
	"io"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runJob is the child's side of a background job that is a process: read the state the
// shell sent over the inherited pipe, run the job, exit with its status.
func (c command) runJob(ctx context.Context, controller *interruptController, argument string) error {
	file, err := runtime.JobStateFile(argument)
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: %v\n", err)
		return exitStatus(2)
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		fmt.Fprintf(c.stderr, "nemosh: job state: %v\n", err)
		return exitStatus(2)
	}
	rt := c.newRuntime()
	executionCtx, clear := controller.context(ctx)
	defer clear()
	if status := rt.RunJob(executionCtx, data); status != 0 {
		return exitStatus(status)
	}
	return nil
}
