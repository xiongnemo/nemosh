package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
)

type pipelineStageRun func(context.Context, Runtime, int) lineResult

type tokenPipelineStage struct {
	runtime Runtime
	run     pipelineStageRun
	// inShell is the last stage under lastpipe, whose runtime is the shell's own; see
	// lastpipe.go. Its jobs are the shell's, and are left running when it ends.
	inShell bool
}

type tokenPipeline struct {
	stages    []tokenPipelineStage
	endpoints []*pipelineEndpoint
}

type pipelineEndpoint struct {
	file *os.File
	once sync.Once
	err  error
	// turn is the `set -x` turn of the stage at this end, passed on before a read that would
	// wait and a write the pipe might not hold; see trace_turn.go.
	turn    *traceTurn
	written atomic.Int64
	// peer is the write end, for a read end; consumed is whether anything was read at it.
	// Together they decide when a read end closes; see lingers.
	peer     *pipelineEndpoint
	consumed atomic.Bool
	closed   chan struct{}
}

func (e *pipelineEndpoint) Read(buffer []byte) (int, error) {
	if e.turn != nil && e.turn.holding() && !inputReady(e.file) {
		e.turn.passOn()
	}
	read, err := e.file.Read(buffer)
	if read > 0 {
		e.consumed.Store(true)
	}
	return read, err
}

// SyscallConn reaches the pipe's descriptor, which `read -t 0` asks whether anything is
// waiting; see read_ready.go.
func (e *pipelineEndpoint) SyscallConn() (syscall.RawConn, error) { return e.file.SyscallConn() }
func (e *pipelineEndpoint) Write(buffer []byte) (int, error) {
	if e.turn != nil && e.written.Add(int64(len(buffer))) > traceTurnPipeBytes {
		e.turn.passOn()
	}
	written, err := e.file.Write(buffer)
	return written, pipelineWriteError(err)
}
func (e *pipelineEndpoint) Close() error {
	if e.lingers() {
		go e.closeLater()
		return nil
	}
	return e.closeNow()
}

func (e *pipelineEndpoint) closeNow() error {
	e.once.Do(func() {
		e.err = errors.Join(interruptPipeIO(e.file), e.file.Close())
		close(e.closed)
	})
	return e.err
}

func newPipelineEndpoints(reader, writer *os.File, readTurn, writeTurn *traceTurn) (*pipelineEndpoint, *pipelineEndpoint) {
	write := &pipelineEndpoint{file: writer, turn: writeTurn, closed: make(chan struct{})}
	read := &pipelineEndpoint{file: reader, turn: readTurn, peer: write, closed: make(chan struct{})}
	return read, write
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
		stage, inShell, err := r.stageRuntime(ctx, index == len(runs)-1)
		if err != nil {
			return tokenPipeline{}, errors.Join(err, closeTokenPipelineStages(stages[:index]))
		}
		stages[index] = tokenPipelineStage{runtime: stage, run: run, inShell: inShell}
	}
	if r.options.xtrace || r.traceTurn != nil {
		for index, turn := range newTraceTurns(len(stages), r.traceTurn) {
			stages[index].runtime.traceTurn = turn
		}
	}
	pipeline := tokenPipeline{stages: stages, endpoints: make([]*pipelineEndpoint, 0, 2*(len(stages)-1))}
	for index := 0; index < len(stages)-1; index++ {
		reader, writer, err := os.Pipe()
		if err != nil {
			return tokenPipeline{}, errors.Join(err, pipeline.closeEndpoints(), closeTokenPipelineStages(stages))
		}
		readEndpoint, writeEndpoint := newPipelineEndpoints(reader, writer, stages[index+1].runtime.traceTurn, stages[index].runtime.traceTurn)
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
		go func() { closeErrors <- endpoint.closeNow() }()
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
			if !stages[index].inShell {
				stages[index].runtime.jobScope.cancelAndDrain()
			}
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
			// A stage is a shell of its own, which a write into a pipe no one reads ends; one
			// run in the shell under lastpipe is the shell's. See pipe_stage.go.
			stageCtx, release := ctx, func() {}
			if !stage.inShell {
				stageCtx, stage.runtime, release = stage.runtime.ownStage(ctx, false)
			}
			result := stage.runtime.guardedRun("running a pipeline stage", func() lineResult {
				return stage.run(stageCtx, stage.runtime, savedStatus)
			})
			result.status = abandonedStatus(ctx, stageCtx, result.status)
			release()
			stage.runtime.traceTurn.passOn()
			if !stage.inShell {
				stage.runtime.jobScope.cancelAndDrain()
			}
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
	statuses := make([]int, 0, len(results))
	for _, result := range results {
		statuses = append(statuses, result.status)
	}
	r.recordPipeStatus(statuses...)
	// An exit, return or break in a stage that ran in the shell is the shell's.
	if last := results[len(results)-1]; last.control != flowNone && pipeline.stages[len(results)-1].inShell {
		return last
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
