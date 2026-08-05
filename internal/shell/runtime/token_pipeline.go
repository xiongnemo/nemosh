package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
)

type pipelineStageRun func(context.Context, Runtime, int) lineResult

type tokenPipelineStage struct {
	runtime Runtime
	run     pipelineStageRun
}

type tokenPipeline struct {
	stages    []tokenPipelineStage
	endpoints []*pipelineEndpoint
}

type pipelineEndpoint struct {
	file *os.File
	once sync.Once
	err  error
}

func (e *pipelineEndpoint) Read(buffer []byte) (int, error) { return e.file.Read(buffer) }
func (e *pipelineEndpoint) Write(buffer []byte) (int, error) {
	written, err := e.file.Write(buffer)
	return written, normalizePipelineWriteError(err)
}
func (e *pipelineEndpoint) Close() error {
	e.once.Do(func() { e.err = errors.Join(interruptPipeIO(e.file), e.file.Close()) })
	return e.err
}

func (r Runtime) prepareTokenPipeline(ctx context.Context, commands [][]shellToken) (tokenPipeline, error) {
	runs := make([]pipelineStageRun, len(commands))
	for index, command := range commands {
		command := command
		runs[index] = func(ctx context.Context, stage Runtime, status int) lineResult {
			return stage.runTokenCommand(ctx, command, status)
		}
	}
	return r.preparePipeline(ctx, runs)
}

func (r Runtime) preparePipeline(ctx context.Context, runs []pipelineStageRun) (tokenPipeline, error) {
	stages := make([]tokenPipelineStage, len(runs))
	for index, run := range runs {
		stage, err := r.snapshot(ctx)
		if err != nil {
			return tokenPipeline{}, errors.Join(err, closeTokenPipelineStages(stages[:index]))
		}
		stages[index] = tokenPipelineStage{runtime: stage, run: run}
	}
	pipeline := tokenPipeline{stages: stages, endpoints: make([]*pipelineEndpoint, 0, 2*(len(stages)-1))}
	for index := 0; index < len(stages)-1; index++ {
		reader, writer, err := os.Pipe()
		if err != nil {
			return tokenPipeline{}, errors.Join(err, pipeline.closeEndpoints(), closeTokenPipelineStages(stages))
		}
		readEndpoint := &pipelineEndpoint{file: reader}
		writeEndpoint := &pipelineEndpoint{file: writer}
		pipeline.endpoints = append(pipeline.endpoints, readEndpoint, writeEndpoint)
		if err := stages[index].runtime.fds.bindOwnedWriter(1, writeEndpoint); err != nil {
			return tokenPipeline{}, errors.Join(err, pipeline.closeEndpoints(), closeTokenPipelineStages(stages))
		}
		if err := stages[index+1].runtime.fds.bindOwnedReader(0, readEndpoint); err != nil {
			return tokenPipeline{}, errors.Join(err, pipeline.closeEndpoints(), closeTokenPipelineStages(stages))
		}
	}
	return pipeline, nil
}

func (p tokenPipeline) closeEndpoints() error {
	closeErrors := make(chan error, len(p.endpoints))
	for _, endpoint := range p.endpoints {
		go func() { closeErrors <- endpoint.Close() }()
	}
	var closeErr error
	for range p.endpoints {
		closeErr = errors.Join(closeErr, <-closeErrors)
	}
	return closeErr
}

func closeTokenPipelineStages(stages []tokenPipelineStage) error {
	var closeErr error
	for index := range stages {
		if stages[index].runtime.fds != nil {
			stages[index].runtime.jobScope.cancelAndDrain()
			closeErr = errors.Join(closeErr, stages[index].runtime.fds.closeAll())
		}
	}
	return closeErr
}

func (r Runtime) executeTokenPipeline(ctx context.Context, pipeline tokenPipeline, savedStatus int) lineResult {
	results := make([]lineResult, len(pipeline.stages))
	var wait sync.WaitGroup
	wait.Add(len(pipeline.stages))
	for index := range pipeline.stages {
		go func() {
			defer wait.Done()
			stage := pipeline.stages[index]
			result := stage.run(ctx, stage.runtime, savedStatus)
			stage.runtime.jobScope.cancelAndDrain()
			if err := stage.runtime.fds.closeAll(); err != nil && result.status == 0 {
				fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
				result.status = 1
			}
			results[index] = result
		}()
	}
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		if err := pipeline.closeEndpoints(); err != nil {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		}
		<-done
	case <-done:
	}
	status := results[len(results)-1].status
	if r.options.pipefail {
		for index := len(results) - 1; index >= 0; index-- {
			if results[index].status != 0 {
				status = results[index].status
				break
			}
		}
	}
	return lineResult{status: status}
}
