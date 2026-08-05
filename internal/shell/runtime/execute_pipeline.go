package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) executeTypedPipeline(ctx context.Context, value pipeline, savedStatus int) lineResult {
	if len(value.commands) == 1 {
		return r.executeCommandNode(ctx, value.commands[0], savedStatus)
	}
	stages := make([]pipelineStageRun, len(value.commands))
	for index, command := range value.commands {
		command := command
		stages[index] = func(ctx context.Context, stage Runtime, status int) lineResult {
			return stage.executeCommandNode(ctx, command, status)
		}
	}
	prepared, err := r.preparePipeline(ctx, stages)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	return r.executeTokenPipeline(ctx, prepared, savedStatus)
}
