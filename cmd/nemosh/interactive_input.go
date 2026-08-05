package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

type inputDemandKind uint8

const (
	inputDemandRead inputDemandKind = iota
	inputDemandLine
)

type interactiveInput struct {
	source   interactiveSource
	demands  chan inputDemand
	leases   chan inputLeaseDemand
	resets   chan chan struct{}
	shutdown chan chan error
}

type inputDemand struct {
	kind        inputDemandKind
	ctx         context.Context
	buffer      []byte
	accumulated int
	accepted    chan struct{}
	result      chan interactiveLine
}

type inputLeaseDemand struct {
	ctx     context.Context
	release <-chan struct{}
	result  chan inputLease
}

type inputLease struct {
	file      *os.File
	available bool
}

type interactiveLine struct {
	count int
	text  string
	err   error
}

func newInteractiveInput(reader io.Reader) *interactiveInput {
	input := &interactiveInput{
		source:   newInteractiveSource(reader),
		demands:  make(chan inputDemand),
		leases:   make(chan inputLeaseDemand),
		resets:   make(chan chan struct{}),
		shutdown: make(chan chan error),
	}
	go input.serve()
	return input
}

func (i *interactiveInput) Read(buffer []byte) (int, error) {
	return i.ReadContext(context.Background(), buffer)
}

func (i *interactiveInput) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	result := make(chan interactiveLine, 1)
	demand := inputDemand{
		kind: inputDemandRead, ctx: ctx, buffer: buffer,
		accepted: make(chan struct{}), result: result,
	}
	if !i.submit(ctx, demand) {
		return 0, ctx.Err()
	}
	read := <-result
	return read.count, read.err
}

func (i *interactiveInput) readLine(ctx context.Context, accumulated int) <-chan interactiveLine {
	result := make(chan interactiveLine, 1)
	demand := inputDemand{
		kind: inputDemandLine, ctx: ctx, accumulated: accumulated,
		accepted: make(chan struct{}), result: result,
	}
	if !i.submit(ctx, demand) {
		result <- interactiveLine{err: ctx.Err()}
	}
	return result
}

func (i *interactiveInput) submit(ctx context.Context, demand inputDemand) bool {
	select {
	case i.demands <- demand:
		<-demand.accepted
		return true
	case <-ctx.Done():
		return false
	}
}

func (i *interactiveInput) reset() {
	acknowledged := make(chan struct{})
	i.resets <- acknowledged
	<-acknowledged
}

func (i *interactiveInput) LeaseStdinFile(ctx context.Context) (*os.File, func(), bool) {
	released := make(chan struct{})
	result := make(chan inputLease, 1)
	demand := inputLeaseDemand{ctx: ctx, release: released, result: result}
	select {
	case i.leases <- demand:
	case <-ctx.Done():
		return nil, func() {}, false
	}
	lease := <-result
	if !lease.available {
		return nil, func() {}, false
	}
	var once sync.Once
	release := func() { once.Do(func() { close(released) }) }
	return lease.file, release, true
}

func (i *interactiveInput) close() error {
	result := make(chan error, 1)
	i.shutdown <- result
	return <-result
}

func (i *interactiveInput) serve() {
	requests := make(chan inputDemand)
	completed := make(chan interactiveLine)
	workerDone := make(chan struct{})
	go i.serveReads(requests, completed, workerDone)
	defer func() {
		close(requests)
		<-workerDone
	}()
	for {
		select {
		case demand := <-i.demands:
			ctx, cancel := context.WithCancel(demand.ctx)
			demand.ctx = ctx
			requests <- demand
			close(demand.accepted)
			if i.awaitDemand(demand, cancel, completed) {
				return
			}
		case demand := <-i.leases:
			file := i.source.File()
			demand.result <- inputLease{file: file, available: file != nil}
			if file != nil {
				<-demand.release
			}
		case acknowledged := <-i.resets:
			close(acknowledged)
		case result := <-i.shutdown:
			result <- i.source.Close()
			return
		}
	}
}

func (i *interactiveInput) awaitDemand(
	demand inputDemand,
	cancel context.CancelFunc,
	completed <-chan interactiveLine,
) bool {
	for {
		select {
		case result := <-completed:
			cancel()
			demand.result <- result
			return false
		case acknowledged := <-i.resets:
			cancel()
			<-completed
			demand.result <- interactiveLine{err: demand.ctx.Err()}
			close(acknowledged)
			return false
		case shutdown := <-i.shutdown:
			cancel()
			<-completed
			demand.result <- interactiveLine{err: demand.ctx.Err()}
			shutdown <- i.source.Close()
			return true
		}
	}
}

func (i *interactiveInput) serveReads(
	requests <-chan inputDemand,
	completed chan<- interactiveLine,
	done chan<- struct{},
) {
	defer close(done)
	for demand := range requests {
		completed <- i.executeDemand(demand.ctx, demand)
	}
}

func (i *interactiveInput) executeDemand(ctx context.Context, demand inputDemand) interactiveLine {
	if demand.kind == inputDemandRead {
		count, err := i.source.ReadContext(ctx, demand.buffer)
		return interactiveLine{count: count, err: err}
	}
	return readInteractiveLine(ctx, i.source, demand.accumulated)
}

func readInteractiveLine(ctx context.Context, source interactiveSource, accumulated int) interactiveLine {
	var line strings.Builder
	emptyReads := 0
	buffer := []byte{0}
	for {
		count, err := source.ReadContext(ctx, buffer)
		if count > 0 {
			emptyReads = 0
			if !runtime.InputSizeAllowed(accumulated + line.Len() + 1) {
				return interactiveLine{err: errInputTooLarge}
			}
			line.WriteByte(buffer[0])
			if buffer[0] == '\n' {
				return interactiveLine{text: line.String(), err: err}
			}
		} else if err == nil {
			emptyReads++
			if emptyReads >= 100 {
				return interactiveLine{err: io.ErrNoProgress}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return interactiveLine{text: line.String(), err: io.EOF}
			}
			return interactiveLine{err: err}
		}
	}
}
