package runtime

import (
	"context"
	"fmt"
)

func (r Runtime) executeTypedPipeline(ctx context.Context, value pipeline, savedStatus int) lineResult {
	result := r.executeTypedPipelineStages(ctx, value, savedStatus)
	// POSIX 2.9.2: `!` gives the logical NOT of the pipeline's exit status. A
	// control transfer is not a status, so `! exit 3` still exits with 3.
	if value.negated && result.control == flowNone {
		if result.status == 0 {
			result.status = 1
		} else {
			result.status = 0
		}
	}
	return result
}

func (r Runtime) executeTypedPipelineStages(ctx context.Context, value pipeline, savedStatus int) lineResult {
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
